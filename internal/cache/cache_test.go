package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time          { return c.t }
func (c *fakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }
func newFakeClock() *fakeClock               { return &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)} }

func TestMemory_TTL(t *testing.T) {
	ctx := context.Background()
	clk := newFakeClock()
	c := NewMemory(clk.Now)

	if _, err := c.Get(ctx, "k"); !errors.Is(err, ErrMiss) {
		t.Fatalf("empty cache: got %v, want ErrMiss", err)
	}
	if err := c.Set(ctx, "k", []byte("v"), time.Minute); err != nil {
		t.Fatal(err)
	}
	got, err := c.Get(ctx, "k")
	if err != nil || string(got) != "v" {
		t.Fatalf("Get = %q, %v", got, err)
	}

	clk.Advance(59 * time.Second)
	if _, err := c.Get(ctx, "k"); err != nil {
		t.Fatalf("not yet expired: %v", err)
	}
	clk.Advance(time.Second)
	if _, err := c.Get(ctx, "k"); !errors.Is(err, ErrMiss) {
		t.Fatalf("expired: got %v, want ErrMiss", err)
	}
	if c.Len() != 0 {
		t.Fatalf("expired entry should be evicted lazily, len=%d", c.Len())
	}
}

func TestMemory_DeleteAndZeroTTL(t *testing.T) {
	ctx := context.Background()
	c := NewMemory(nil)
	_ = c.Set(ctx, "a", []byte("1"), time.Hour)
	_ = c.Set(ctx, "b", []byte("2"), time.Hour)
	if err := c.Delete(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get(ctx, "a"); !errors.Is(err, ErrMiss) {
		t.Fatalf("deleted key: got %v", err)
	}
	_ = c.Set(ctx, "b", []byte("3"), 0)
	if _, err := c.Get(ctx, "b"); !errors.Is(err, ErrMiss) {
		t.Fatalf("zero ttl must delete: got %v", err)
	}
}

func TestMemory_CopiesValues(t *testing.T) {
	ctx := context.Background()
	c := NewMemory(nil)
	src := []byte("hello")
	_ = c.Set(ctx, "k", src, time.Hour)
	src[0] = 'j'
	got, _ := c.Get(ctx, "k")
	if string(got) != "hello" {
		t.Fatalf("cache must not alias caller's slice: %q", got)
	}
	got[0] = 'x'
	again, _ := c.Get(ctx, "k")
	if string(again) != "hello" {
		t.Fatalf("returned slice must not alias stored value: %q", again)
	}
}

func TestMemory_Sweep(t *testing.T) {
	ctx := context.Background()
	clk := newFakeClock()
	c := NewMemory(clk.Now)
	for i := 0; i < sweepEvery-1; i++ {
		_ = c.Set(ctx, string(rune('a'+i%26))+string(rune(i)), []byte("x"), time.Second)
	}
	clk.Advance(2 * time.Second)
	_ = c.Set(ctx, "fresh", []byte("x"), time.Hour)
	if c.Len() != 1 {
		t.Fatalf("sweep should drop expired entries, len=%d", c.Len())
	}
}

func TestRedis(t *testing.T) {
	ctx := context.Background()
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	c := NewRedis(client)
	defer c.Close()

	if err := c.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get(ctx, "k"); !errors.Is(err, ErrMiss) {
		t.Fatalf("empty: got %v", err)
	}
	if err := c.Set(ctx, "k", []byte("v"), time.Minute); err != nil {
		t.Fatal(err)
	}
	got, err := c.Get(ctx, "k")
	if err != nil || string(got) != "v" {
		t.Fatalf("Get = %q, %v", got, err)
	}
	srv.FastForward(2 * time.Minute)
	if _, err := c.Get(ctx, "k"); !errors.Is(err, ErrMiss) {
		t.Fatalf("expired: got %v", err)
	}
	_ = c.Set(ctx, "d", []byte("v"), time.Minute)
	if err := c.Delete(ctx, "d"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get(ctx, "d"); !errors.Is(err, ErrMiss) {
		t.Fatalf("deleted: got %v", err)
	}
	_ = c.Set(ctx, "z", []byte("v"), time.Minute)
	_ = c.Set(ctx, "z", []byte("v"), 0)
	if _, err := c.Get(ctx, "z"); !errors.Is(err, ErrMiss) {
		t.Fatalf("zero ttl: got %v", err)
	}
}
