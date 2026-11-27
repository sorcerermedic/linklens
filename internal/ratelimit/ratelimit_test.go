package ratelimit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time          { return c.t }
func (c *fakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

func newClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

type limiterFactory func(t *testing.T, cfg Config, clk *fakeClock) Limiter

func memoryFactory(_ *testing.T, cfg Config, clk *fakeClock) Limiter {
	return NewMemory(cfg, clk.Now)
}

func redisFactory(t *testing.T, cfg Config, clk *fakeClock) Limiter {
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedis(client, cfg, clk.Now)
}

func TestLimiters(t *testing.T) {
	backends := map[string]limiterFactory{"memory": memoryFactory, "redis": redisFactory}

	type step struct {
		advance   time.Duration
		key       string
		wantAllow bool
		wantLeft  int
	}
	scenarios := []struct {
		name  string
		cfg   Config
		steps []step
	}{
		{
			name: "burst then refill",
			cfg:  Config{Rate: 1, Burst: 3},
			steps: []step{
				{key: "a", wantAllow: true, wantLeft: 2},
				{key: "a", wantAllow: true, wantLeft: 1},
				{key: "a", wantAllow: true, wantLeft: 0},
				{key: "a", wantAllow: false, wantLeft: 0},
				{advance: 500 * time.Millisecond, key: "a", wantAllow: false, wantLeft: 0},
				{advance: 500 * time.Millisecond, key: "a", wantAllow: true, wantLeft: 0},
				{advance: 10 * time.Second, key: "a", wantAllow: true, wantLeft: 2},
			},
		},
		{
			name: "keys are independent",
			cfg:  Config{Rate: 1, Burst: 1},
			steps: []step{
				{key: "a", wantAllow: true, wantLeft: 0},
				{key: "b", wantAllow: true, wantLeft: 0},
				{key: "a", wantAllow: false},
				{key: "b", wantAllow: false},
			},
		},
		{
			name: "high rate",
			cfg:  Config{Rate: 100, Burst: 2},
			steps: []step{
				{key: "a", wantAllow: true, wantLeft: 1},
				{key: "a", wantAllow: true, wantLeft: 0},
				{key: "a", wantAllow: false},
				{advance: 10 * time.Millisecond, key: "a", wantAllow: true, wantLeft: 0},
				{advance: 20 * time.Millisecond, key: "a", wantAllow: true, wantLeft: 1},
			},
		},
	}

	for backend, factory := range backends {
		for _, sc := range scenarios {
			t.Run(backend+"/"+sc.name, func(t *testing.T) {
				clk := newClock()
				l := factory(t, sc.cfg, clk)
				for i, st := range sc.steps {
					clk.Advance(st.advance)
					d, err := l.Allow(context.Background(), st.key)
					if err != nil {
						t.Fatalf("step %d: %v", i, err)
					}
					if d.Allowed != st.wantAllow {
						t.Fatalf("step %d: allowed=%v want %v (%+v)", i, d.Allowed, st.wantAllow, d)
					}
					if d.Allowed && d.Remaining != st.wantLeft {
						t.Fatalf("step %d: remaining=%d want %d", i, d.Remaining, st.wantLeft)
					}
					if !d.Allowed && d.RetryAfter <= 0 {
						t.Fatalf("step %d: rejected request must carry a positive RetryAfter: %+v", i, d)
					}
				}
			})
		}
	}
}

func TestMemory_RetryAfter(t *testing.T) {
	clk := newClock()
	l := NewMemory(Config{Rate: 2, Burst: 1}, clk.Now)
	_, _ = l.Allow(context.Background(), "k")
	d, _ := l.Allow(context.Background(), "k")
	if d.Allowed || d.RetryAfter != 500*time.Millisecond {
		t.Fatalf("want 500ms retry, got %+v", d)
	}
}

func TestMemory_GC(t *testing.T) {
	clk := newClock()
	l := NewMemory(Config{Rate: 1, Burst: 1}, clk.Now)
	for i := 0; i < gcEvery-1; i++ {
		_, _ = l.Allow(context.Background(), "k"+string(rune(i)))
	}
	clk.Advance(time.Hour)
	_, _ = l.Allow(context.Background(), "fresh")
	if l.Len() != 1 {
		t.Fatalf("idle buckets should be collected, len=%d", l.Len())
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		cfg     Config
		wantErr bool
	}{
		{Config{Rate: 1, Burst: 1}, false},
		{Config{Rate: 0, Burst: 1}, true},
		{Config{Rate: -1, Burst: 1}, true},
		{Config{Rate: 1, Burst: 0}, true},
	}
	for _, tc := range tests {
		if err := tc.cfg.Validate(); (err != nil) != tc.wantErr {
			t.Errorf("%+v: err=%v wantErr=%v", tc.cfg, err, tc.wantErr)
		}
	}
}

type errLimiter struct{}

func (errLimiter) Allow(context.Context, string) (Decision, error) {
	return Decision{}, errors.New("boom")
}

func TestMiddleware(t *testing.T) {
	clk := newClock()
	limited := 0
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := Middleware(NewMemory(Config{Rate: 1, Burst: 2}, clk.Now), MiddlewareOptions{
		Limit:     2,
		OnLimited: func() { limited++ },
	})(ok)

	do := func(addr string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = addr
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	if rec := do("10.0.0.1:1234"); rec.Code != 200 || rec.Header().Get("X-RateLimit-Remaining") != "1" {
		t.Fatalf("first request: %d %v", rec.Code, rec.Header())
	}
	if rec := do("10.0.0.1:9999"); rec.Code != 200 || rec.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Fatalf("second request (same ip, other port): %d", rec.Code)
	}
	rec := do("10.0.0.1:1234")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("third request: %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") != "1" || rec.Header().Get("X-RateLimit-Limit") != "2" {
		t.Fatalf("headers: %v", rec.Header())
	}
	if limited != 1 {
		t.Fatalf("OnLimited called %d times", limited)
	}
	if rec := do("10.0.0.2:1"); rec.Code != 200 {
		t.Fatalf("other ip must not be limited: %d", rec.Code)
	}

	t.Run("fails open on backend error", func(t *testing.T) {
		h := Middleware(errLimiter{}, MiddlewareOptions{})(ok)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != 200 {
			t.Fatalf("expected fail-open 200, got %d", rec.Code)
		}
	})
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remote     string
		xff, xrip  string
		trustProxy bool
		want       string
	}{
		{"peer only", "203.0.113.5:4321", "", "", false, "203.0.113.5"},
		{"ignores xff when untrusted", "203.0.113.5:4321", "198.51.100.1", "", false, "203.0.113.5"},
		{"xff first hop", "10.0.0.1:1", "198.51.100.1, 10.0.0.9", "", true, "198.51.100.1"},
		{"xff single", "10.0.0.1:1", " 198.51.100.2 ", "", true, "198.51.100.2"},
		{"x-real-ip", "10.0.0.1:1", "", "198.51.100.3", true, "198.51.100.3"},
		{"ipv6 peer", "[2001:db8::1]:443", "", "", false, "2001:db8::1"},
		{"malformed remote", "garbage", "", "", false, "garbage"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remote
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			if tc.xrip != "" {
				req.Header.Set("X-Real-IP", tc.xrip)
			}
			if got := ClientIP(tc.trustProxy)(req); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
