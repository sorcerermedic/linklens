package cache

import (
	"context"
	"sync"
	"time"
)

const sweepEvery = 1024

type entry struct {
	value     []byte
	expiresAt time.Time
}

type Memory struct {
	mu    sync.Mutex
	items map[string]entry
	now   func() time.Time
	sets  int
}

func NewMemory(clock func() time.Time) *Memory {
	if clock == nil {
		clock = time.Now
	}
	return &Memory{items: make(map[string]entry), now: clock}
}

func (m *Memory) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.items[key]
	if !ok {
		return nil, ErrMiss
	}
	if !m.now().Before(e.expiresAt) {
		delete(m.items, key)
		return nil, ErrMiss
	}
	out := make([]byte, len(e.value))
	copy(out, e.value)
	return out, nil
}

func (m *Memory) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ttl <= 0 {
		delete(m.items, key)
		return nil
	}
	stored := make([]byte, len(value))
	copy(stored, value)
	m.items[key] = entry{value: stored, expiresAt: m.now().Add(ttl)}
	m.sets++
	if m.sets%sweepEvery == 0 {
		m.sweepLocked()
	}
	return nil
}

func (m *Memory) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, key)
	return nil
}

func (m *Memory) Ping(context.Context) error { return nil }

func (m *Memory) Close() error { return nil }

func (m *Memory) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.items)
}

func (m *Memory) sweepLocked() {
	now := m.now()
	for k, e := range m.items {
		if !now.Before(e.expiresAt) {
			delete(m.items, k)
		}
	}
}
