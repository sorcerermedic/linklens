package analytics

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/portfolio-author/linklens/internal/store"
)

type Sink interface {
	RecordClicks(ctx context.Context, clicks []store.Click) error
}

type Observer interface {
	ClicksRecorded(n int)
	ClickDropped()
}

type noopObserver struct{}

func (noopObserver) ClicksRecorded(int) {}
func (noopObserver) ClickDropped()      {}

type RecorderOptions struct {
	BufferSize int

	BatchSize int

	FlushInterval time.Duration

	WriteTimeout time.Duration
	Observer     Observer
	Logger       *slog.Logger
}

func (o *RecorderOptions) setDefaults() {
	if o.BufferSize <= 0 {
		o.BufferSize = 1024
	}
	if o.BatchSize <= 0 {
		o.BatchSize = 100
	}
	if o.FlushInterval <= 0 {
		o.FlushInterval = time.Second
	}
	if o.WriteTimeout <= 0 {
		o.WriteTimeout = 5 * time.Second
	}
	if o.Observer == nil {
		o.Observer = noopObserver{}
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
}

type Recorder struct {
	opts    RecorderOptions
	sink    Sink
	events  chan Event
	done    chan struct{}
	closeMu sync.Mutex
	closed  bool
	dropped atomic.Int64
}

func NewRecorder(sink Sink, opts RecorderOptions) *Recorder {
	opts.setDefaults()
	r := &Recorder{
		opts:   opts,
		sink:   sink,
		events: make(chan Event, opts.BufferSize),
		done:   make(chan struct{}),
	}
	go r.run()
	return r
}

func (r *Recorder) Record(ev Event) bool {
	r.closeMu.Lock()
	defer r.closeMu.Unlock()
	if r.closed {
		return false
	}
	select {
	case r.events <- ev:
		return true
	default:
		r.dropped.Add(1)
		r.opts.Observer.ClickDropped()
		return false
	}
}

func (r *Recorder) Dropped() int64 { return r.dropped.Load() }

func (r *Recorder) Close(ctx context.Context) error {
	r.closeMu.Lock()
	if !r.closed {
		r.closed = true
		close(r.events)
	}
	r.closeMu.Unlock()

	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Recorder) run() {
	defer close(r.done)
	ticker := time.NewTicker(r.opts.FlushInterval)
	defer ticker.Stop()

	batch := make([]store.Click, 0, r.opts.BatchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), r.opts.WriteTimeout)
		err := r.sink.RecordClicks(ctx, batch)
		cancel()
		if err != nil {
			r.opts.Logger.Error("failed to record clicks", "count", len(batch), "error", err)
		} else {
			r.opts.Observer.ClicksRecorded(len(batch))
		}
		batch = batch[:0]
	}

	for {
		select {
		case ev, ok := <-r.events:
			if !ok {
				flush()
				return
			}
			batch = append(batch, Enrich(ev))
			if len(batch) >= r.opts.BatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}
