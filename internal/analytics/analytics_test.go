package analytics

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/portfolio-author/linklens/internal/store"
)

func TestClassifyDevice(t *testing.T) {
	tests := []struct {
		name, ua, want string
	}{
		{"empty", "", DeviceUnknown},
		{"chrome windows", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36", DeviceDesktop},
		{"safari mac", "Mozilla/5.0 (Macintosh; Intel Mac OS X 13_5) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.5 Safari/605.1.15", DeviceDesktop},
		{"firefox linux", "Mozilla/5.0 (X11; Linux x86_64; rv:120.0) Gecko/20100101 Firefox/120.0", DeviceDesktop},
		{"chromeos", "Mozilla/5.0 (X11; CrOS x86_64 14541.0.0) AppleWebKit/537.36 Chrome/120.0 Safari/537.36", DeviceDesktop},
		{"iphone", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148 Safari/604.1", DeviceMobile},
		{"android phone", "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 Chrome/120.0 Mobile Safari/537.36", DeviceMobile},
		{"android tablet", "Mozilla/5.0 (Linux; Android 13; SM-X700) AppleWebKit/537.36 Chrome/120.0 Safari/537.36", DeviceTablet},
		{"ipad", "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148 Safari/604.1", DeviceTablet},
		{"kindle", "Mozilla/5.0 (Linux; U; Android 4.4.3; KFTHWI) Silk/44.1.54 like Chrome/44.0 Safari/537.36", DeviceTablet},
		{"googlebot", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", DeviceBot},
		{"curl", "curl/8.4.0", DeviceBot},
		{"go client", "Go-http-client/1.1", DeviceBot},
		{"python", "python-requests/2.31", DeviceBot},
		{"slack preview", "Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots)", DeviceBot},
		{"gibberish", "SomethingNew/1.0", DeviceUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyDevice(tc.ua); got != tc.want {
				t.Fatalf("ClassifyDevice(%q) = %q, want %q", tc.ua, got, tc.want)
			}
		})
	}
}

func TestLookupCountry(t *testing.T) {
	tests := []struct {
		name, hint, ip, want string
	}{
		{"header wins", "TR", "8.8.8.8", "TR"},
		{"header lowercased", "de", "", "DE"},
		{"header XX ignored", "XX", "8.8.8.8", "US"},
		{"header T1 ignored", "T1", "", CountryUnknown},
		{"bad header length", "USA", "", CountryUnknown},
		{"bad header chars", "U1", "", CountryUnknown},
		{"google dns", "", "8.8.4.4", "US"},
		{"cloudflare dns", "", "1.1.1.1", "AU"},
		{"turk telekom", "", "195.175.39.39", "TR"},
		{"google ipv6", "", "2001:4860:4860::8888", "US"},
		{"unknown public", "", "203.0.113.7", CountryUnknown},
		{"private", "", "10.1.2.3", CountryUnknown},
		{"loopback", "", "127.0.0.1", CountryUnknown},
		{"garbage", "", "not-an-ip", CountryUnknown},
		{"empty", "", "", CountryUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := LookupCountry(tc.hint, tc.ip); got != tc.want {
				t.Fatalf("LookupCountry(%q, %q) = %q, want %q", tc.hint, tc.ip, got, tc.want)
			}
		})
	}
}

func TestNormalizeReferrer(t *testing.T) {
	tests := map[string]string{
		"":                                       ReferrerDirect,
		"   ":                                    ReferrerDirect,
		"https://news.ycombinator.com/item?id=1": "news.ycombinator.com",
		"https://www.Google.com/search?q=x":      "google.com",
		"http://t.co/abc":                        "t.co",
		"not a url":                              ReferrerDirect,
		"android-app://com.slack/":               "com.slack",
	}
	for in, want := range tests {
		if got := NormalizeReferrer(in); got != want {
			t.Errorf("NormalizeReferrer(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEnrich(t *testing.T) {
	at := time.Date(2026, 5, 6, 7, 8, 9, 0, time.FixedZone("x", 3*3600))
	c := Enrich(Event{
		Code:      "abcd",
		At:        at,
		IP:        "8.8.8.8",
		UserAgent: "Mozilla/5.0 (iPhone) Mobile Safari",
		Referrer:  "https://www.example.org/page",
	})
	want := store.Click{Code: "abcd", At: at.UTC(), Country: "US", Device: DeviceMobile, Referrer: "example.org"}
	if c != want {
		t.Fatalf("Enrich = %+v, want %+v", c, want)
	}
	if c.At.Location() != time.UTC {
		t.Fatalf("At must be UTC")
	}
}

type recordingSink struct {
	mu      sync.Mutex
	batches [][]store.Click
	err     error
}

func (s *recordingSink) RecordClicks(_ context.Context, clicks []store.Click) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	cp := make([]store.Click, len(clicks))
	copy(cp, clicks)
	s.batches = append(s.batches, cp)
	return nil
}

func (s *recordingSink) total() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, b := range s.batches {
		n += len(b)
	}
	return n
}

type countingObserver struct {
	recorded atomic64
	dropped  atomic64
}

type atomic64 struct {
	mu sync.Mutex
	n  int64
}

func (a *atomic64) add(d int64) { a.mu.Lock(); a.n += d; a.mu.Unlock() }
func (a *atomic64) get() int64  { a.mu.Lock(); defer a.mu.Unlock(); return a.n }

func (o *countingObserver) ClicksRecorded(n int) { o.recorded.add(int64(n)) }
func (o *countingObserver) ClickDropped()        { o.dropped.add(1) }

func TestRecorder_BatchesAndFlushesOnClose(t *testing.T) {
	sink := &recordingSink{}
	obs := &countingObserver{}
	r := NewRecorder(sink, RecorderOptions{BufferSize: 100, BatchSize: 3, FlushInterval: time.Hour, Observer: obs})

	for i := 0; i < 7; i++ {
		if !r.Record(Event{Code: "c", At: time.Now(), UserAgent: "curl/8"}) {
			t.Fatalf("event %d dropped", i)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := r.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if sink.total() != 7 {
		t.Fatalf("recorded %d clicks, want 7", sink.total())
	}
	if len(sink.batches) != 3 {
		t.Fatalf("got %d batches, want 3: %v", len(sink.batches), sink.batches)
	}
	if sink.batches[0][0].Device != DeviceBot {
		t.Fatalf("events must be enriched before reaching the sink: %+v", sink.batches[0][0])
	}
	if obs.recorded.get() != 7 {
		t.Fatalf("observer recorded=%d", obs.recorded.get())
	}
	if r.Record(Event{}) {
		t.Fatal("Record after Close must return false")
	}
}

func TestRecorder_FlushInterval(t *testing.T) {
	sink := &recordingSink{}
	r := NewRecorder(sink, RecorderOptions{BatchSize: 1000, FlushInterval: 10 * time.Millisecond})
	defer func() { _ = r.Close(context.Background()) }()
	r.Record(Event{Code: "c", At: time.Now()})

	deadline := time.Now().Add(2 * time.Second)
	for sink.total() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if sink.total() != 1 {
		t.Fatalf("partial batch was not flushed by ticker")
	}
}

func TestRecorder_DropsWhenFull(t *testing.T) {

	release := make(chan struct{})
	blocking := &blockingSink{release: release, entered: make(chan struct{})}
	obs := &countingObserver{}
	r := NewRecorder(blocking, RecorderOptions{BufferSize: 2, BatchSize: 1, FlushInterval: time.Hour, Observer: obs})

	r.Record(Event{Code: "first"})
	<-blocking.entered
	accepted := 0
	for i := 0; i < 10; i++ {
		if r.Record(Event{Code: "x"}) {
			accepted++
		}
	}
	if accepted != 2 {
		t.Fatalf("accepted %d events, want exactly the buffer size (2)", accepted)
	}
	if r.Dropped() != 8 || obs.dropped.get() != 8 {
		t.Fatalf("dropped=%d observer=%d, want 8", r.Dropped(), obs.dropped.get())
	}
	close(release)
	if err := r.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

type blockingSink struct {
	release chan struct{}
	entered chan struct{}
	once    sync.Once
}

func (b *blockingSink) RecordClicks(context.Context, []store.Click) error {
	b.once.Do(func() { close(b.entered) })
	<-b.release
	return nil
}

func TestRecorder_SinkErrorIsLoggedNotFatal(t *testing.T) {
	sink := &recordingSink{err: errors.New("db down")}
	r := NewRecorder(sink, RecorderOptions{BatchSize: 1, FlushInterval: time.Hour})
	r.Record(Event{Code: "c"})
	r.Record(Event{Code: "c"})
	if err := r.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRecorder_CloseTimeout(t *testing.T) {
	release := make(chan struct{})
	blocking := &blockingSink{release: release, entered: make(chan struct{})}
	r := NewRecorder(blocking, RecorderOptions{BatchSize: 1, FlushInterval: time.Hour})
	r.Record(Event{Code: "c"})
	<-blocking.entered
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := r.Close(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Close should respect ctx deadline, got %v", err)
	}
	close(release)
	_ = r.Close(context.Background())
}
