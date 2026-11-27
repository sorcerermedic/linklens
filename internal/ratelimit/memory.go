package ratelimit

import (
	"context"
	"sync"
	"time"
)

const gcEvery = 4096

type bucket struct {
	tokens float64
	last   time.Time
}

type Memory struct {
	cfg     Config
	now     Clock
	mu      sync.Mutex
	buckets map[string]*bucket
	calls   int
}

func NewMemory(cfg Config, clock Clock) *Memory {
	if clock == nil {
		clock = time.Now
	}
	return &Memory{cfg: cfg, now: clock, buckets: make(map[string]*bucket)}
}

func (m *Memory) Allow(_ context.Context, key string) (Decision, error) {
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()

	b, ok := m.buckets[key]
	if !ok {
		b = &bucket{tokens: float64(m.cfg.Burst), last: now}
		m.buckets[key] = b
	}
	var d Decision
	b.tokens, d = take(b.tokens, b.last, now, m.cfg)
	b.last = now

	m.calls++
	if m.calls%gcEvery == 0 {
		m.gcLocked(now)
	}
	return d, nil
}

func (m *Memory) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.buckets)
}

func (m *Memory) gcLocked(now time.Time) {
	refill := time.Duration(float64(m.cfg.Burst) / m.cfg.Rate * float64(time.Second))
	for k, b := range m.buckets {
		if now.Sub(b.last) >= refill {
			delete(m.buckets, k)
		}
	}
}
