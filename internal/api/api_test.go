package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/portfolio-author/linklens/internal/analytics"
	"github.com/portfolio-author/linklens/internal/cache"
	"github.com/portfolio-author/linklens/internal/ratelimit"
	"github.com/portfolio-author/linklens/internal/shortener"
	"github.com/portfolio-author/linklens/internal/store"
	"github.com/portfolio-author/linklens/internal/store/memory"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

type fakeRecorder struct {
	mu     sync.Mutex
	events []analytics.Event
}

func (f *fakeRecorder) Record(ev analytics.Event) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, ev)
	return true
}

func (f *fakeRecorder) all() []analytics.Event {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]analytics.Event(nil), f.events...)
}

type countingMetrics struct {
	mu                 sync.Mutex
	redirects, limited int
	routes             map[string]int
}

func (m *countingMetrics) ObserveRequest(method, route string, status int, _ time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.routes == nil {
		m.routes = map[string]int{}
	}
	m.routes[method+" "+route]++
}
func (m *countingMetrics) Redirect()    { m.mu.Lock(); m.redirects++; m.mu.Unlock() }
func (m *countingMetrics) RateLimited() { m.mu.Lock(); m.limited++; m.mu.Unlock() }

type env struct {
	handler  http.Handler
	store    *memory.Store
	recorder *fakeRecorder
	metrics  *countingMetrics
	now      time.Time
}

type option func(*Config)

func newEnv(t *testing.T, opts ...option) *env {
	t.Helper()
	e := &env{
		store:    memory.New(),
		recorder: &fakeRecorder{},
		metrics:  &countingMetrics{},
		now:      time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
	}
	clock := func() time.Time { return e.now }
	svc := shortener.NewService(e.store, cache.NewMemory(clock), shortener.Options{
		Clock: clock, Logger: discard,
	})
	cfg := Config{
		Service:      svc,
		Recorder:     e.recorder,
		BaseURL:      "https://lnk.test",
		Metrics:      e.metrics,
		HealthChecks: map[string]Pinger{"store": e.store},
		Clock:        clock,
		Logger:       discard,
	}
	for _, o := range opts {
		o(&cfg)
	}
	e.handler = New(cfg)
	return e
}

func (e *env) do(method, target, body string, hdr ...string) *httptest.ResponseRecorder {
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, rd)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

func TestCreateLink(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantErr    string
	}{
		{"generated", `{"url":"https://example.com/a"}`, http.StatusCreated, ""},
		{"alias", `{"url":"https://example.com/a","alias":"launch-2026"}`, http.StatusCreated, ""},
		{"with expiry", `{"url":"https://example.com/a","expiresAt":"2026-06-01T00:00:00Z"}`, http.StatusCreated, ""},
		{"bad json", `{"url":`, http.StatusBadRequest, "invalid JSON"},
		{"unknown field", `{"url":"https://example.com","foo":1}`, http.StatusBadRequest, "invalid JSON"},
		{"bad scheme", `{"url":"ftp://example.com"}`, http.StatusBadRequest, "only http and https"},
		{"private host", `{"url":"http://192.168.0.1/admin"}`, http.StatusBadRequest, "private networks"},
		{"bad alias", `{"url":"https://example.com","alias":"no"}`, http.StatusBadRequest, "alias must be"},
		{"reserved alias", `{"url":"https://example.com","alias":"healthz"}`, http.StatusBadRequest, "reserved"},
		{"past expiry", `{"url":"https://example.com","expiresAt":"2020-01-01T00:00:00Z"}`, http.StatusBadRequest, "future"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			rec := e.do(http.MethodPost, "/api/links", tc.body)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tc.wantStatus, rec.Body)
			}
			if tc.wantErr != "" {
				body := decode[map[string]string](t, rec)
				if !strings.Contains(body["error"], tc.wantErr) {
					t.Fatalf("error %q does not mention %q", body["error"], tc.wantErr)
				}
				return
			}
			got := decode[linkResponse](t, rec)
			if got.Code == "" || got.ShortURL != "https://lnk.test/"+got.Code || got.URL != "https://example.com/a" {
				t.Fatalf("unexpected response %+v", got)
			}
			if rec.Header().Get("Location") != "/api/links/"+got.Code+"/stats" {
				t.Fatalf("Location = %q", rec.Header().Get("Location"))
			}
		})
	}
}

func TestCreateLink_AliasConflict(t *testing.T) {
	e := newEnv(t)
	body := `{"url":"https://example.com","alias":"taken-one"}`
	if rec := e.do(http.MethodPost, "/api/links", body); rec.Code != http.StatusCreated {
		t.Fatalf("first create: %d", rec.Code)
	}
	if rec := e.do(http.MethodPost, "/api/links", body); rec.Code != http.StatusConflict {
		t.Fatalf("second create: %d, want 409", rec.Code)
	}
}

func TestCreateLink_BodyTooLarge(t *testing.T) {
	e := newEnv(t)
	body := `{"url":"https://example.com/` + strings.Repeat("a", maxBodyBytes) + `"}`
	if rec := e.do(http.MethodPost, "/api/links", body); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
}

func TestRedirect(t *testing.T) {
	e := newEnv(t)
	created := decode[linkResponse](t, e.do(http.MethodPost, "/api/links", `{"url":"https://example.com/landing?utm=x"}`))

	rec := e.do(http.MethodGet, "/"+created.Code, "",
		"User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) Mobile/15E148",
		"Referer", "https://news.ycombinator.com/item?id=1",
		"CF-IPCountry", "TR")
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "https://example.com/landing?utm=x" {
		t.Fatalf("Location = %q", loc)
	}
	if rec.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("redirect must not be cacheable: %q", rec.Header().Get("Cache-Control"))
	}
	evs := e.recorder.all()
	if len(evs) != 1 {
		t.Fatalf("recorded %d events, want 1", len(evs))
	}
	ev := evs[0]
	if ev.Code != created.Code || ev.CountryHint != "TR" || !strings.Contains(ev.UserAgent, "iPhone") ||
		ev.Referrer != "https://news.ycombinator.com/item?id=1" || !ev.At.Equal(e.now) || ev.IP != "192.0.2.1" {
		t.Fatalf("unexpected event %+v", ev)
	}
	if e.metrics.redirects != 1 {
		t.Fatalf("redirect metric = %d", e.metrics.redirects)
	}
}

func TestRedirect_Errors(t *testing.T) {
	e := newEnv(t)
	exp := e.now.Add(time.Hour).Format(time.RFC3339)
	created := decode[linkResponse](t, e.do(http.MethodPost, "/api/links", `{"url":"https://example.com","expiresAt":"`+exp+`"}`))

	if rec := e.do(http.MethodGet, "/doesnotexist", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown code: %d", rec.Code)
	}
	if rec := e.do(http.MethodGet, "/bad%20code", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("invalid code: %d", rec.Code)
	}
	e.now = e.now.Add(2 * time.Hour)
	if rec := e.do(http.MethodGet, "/"+created.Code, ""); rec.Code != http.StatusGone {
		t.Fatalf("expired link: %d, want 410", rec.Code)
	}
	if n := len(e.recorder.all()); n != 0 {
		t.Fatalf("failed redirects must not record clicks, got %d", n)
	}
}

func TestListLinks(t *testing.T) {
	e := newEnv(t)
	if rec := e.do(http.MethodGet, "/api/links", ""); rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("empty list: %d %s", rec.Code, rec.Body)
	}
	exp := e.now.Add(time.Minute).Format(time.RFC3339)
	e.do(http.MethodPost, "/api/links", `{"url":"https://a.example","expiresAt":"`+exp+`"}`)
	e.now = e.now.Add(time.Second)
	e.do(http.MethodPost, "/api/links", `{"url":"https://b.example","alias":"bbbb"}`)
	e.now = e.now.Add(time.Hour)

	links := decode[[]linkResponse](t, e.do(http.MethodGet, "/api/links", ""))
	if len(links) != 2 || links[0].Code != "bbbb" || !links[0].Custom || links[0].Expired {
		t.Fatalf("unexpected list %+v", links)
	}
	if !links[1].Expired || links[1].ExpiresAt == nil {
		t.Fatalf("first link should be reported expired: %+v", links[1])
	}
}

func TestStatsAndDelete(t *testing.T) {
	e := newEnv(t)
	created := decode[linkResponse](t, e.do(http.MethodPost, "/api/links", `{"url":"https://example.com","alias":"stats-me"}`))
	_ = e.store.RecordClicks(context.Background(), []store.Click{
		{Code: created.Code, At: e.now, Device: "mobile", Country: "TR", Referrer: "direct"},
		{Code: created.Code, At: e.now.Add(24 * time.Hour), Device: "desktop", Country: "US", Referrer: "github.com"},
	})

	rec := e.do(http.MethodGet, "/api/links/stats-me/stats", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("stats: %d", rec.Code)
	}
	st := decode[store.Stats](t, rec)
	if st.TotalClicks != 2 || len(st.ByDay) != 2 || st.ByDevice["mobile"] != 1 || st.ByCountry["US"] != 1 || len(st.TopReferrers) != 2 {
		t.Fatalf("unexpected stats %+v", st)
	}
	if rec := e.do(http.MethodGet, "/api/links/nope/stats", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("missing stats: %d", rec.Code)
	}

	if rec := e.do(http.MethodDelete, "/api/links/stats-me", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	if rec := e.do(http.MethodDelete, "/api/links/stats-me", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("second delete: %d", rec.Code)
	}
	if rec := e.do(http.MethodGet, "/stats-me", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("redirect after delete: %d", rec.Code)
	}
}

func TestUnknownAPIRoute(t *testing.T) {
	e := newEnv(t)
	rec := e.do(http.MethodGet, "/api/nothing", "")
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Header().Get("Content-Type"), "json") {
		t.Fatalf("got %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
}

func TestRateLimit(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	lim := ratelimit.NewMemory(ratelimit.Config{Rate: 1, Burst: 2}, func() time.Time { return now })
	e := newEnv(t, func(c *Config) { c.Limiter = lim; c.RateLimitBurst = 2 })

	for i := 0; i < 2; i++ {
		if rec := e.do(http.MethodGet, "/api/links", ""); rec.Code != http.StatusOK {
			t.Fatalf("request %d: %d", i, rec.Code)
		}
	}
	rec := e.do(http.MethodGet, "/api/links", "")
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("third request: %d %v", rec.Code, rec.Header())
	}
	if e.metrics.limited != 1 {
		t.Fatalf("rate limited metric = %d", e.metrics.limited)
	}

	if rec := e.do(http.MethodGet, "/healthz", ""); rec.Code != http.StatusOK {
		t.Fatalf("healthz: %d", rec.Code)
	}
}

type downPinger struct{}

func (downPinger) Ping(context.Context) error { return errors.New("connection refused") }

func TestHealth(t *testing.T) {
	e := newEnv(t)
	rec := e.do(http.MethodGet, "/healthz", "")
	body := decode[map[string]any](t, rec)
	if rec.Code != http.StatusOK || body["status"] != "ok" {
		t.Fatalf("healthy: %d %v", rec.Code, body)
	}

	e = newEnv(t, func(c *Config) { c.HealthChecks["cache"] = downPinger{} })
	rec = e.do(http.MethodGet, "/healthz", "")
	body = decode[map[string]any](t, rec)
	checks := body["checks"].(map[string]any)
	if rec.Code != http.StatusServiceUnavailable || body["status"] != "degraded" || checks["store"] != "ok" {
		t.Fatalf("degraded: %d %v", rec.Code, body)
	}
}

func TestMetricsRouteAndInstrumentation(t *testing.T) {
	e := newEnv(t, func(c *Config) {
		c.MetricsHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("# metrics"))
		})
	})
	if rec := e.do(http.MethodGet, "/metrics", ""); rec.Code != 200 || rec.Body.String() != "# metrics" {
		t.Fatalf("metrics: %d %s", rec.Code, rec.Body)
	}
	e.do(http.MethodGet, "/abcd", "")
	e.do(http.MethodGet, "/efgh", "")
	if e.metrics.routes["GET /{code}"] != 2 {
		t.Fatalf("redirects must share the route pattern label: %v", e.metrics.routes)
	}
}

func TestDashboard(t *testing.T) {
	web := fstest.MapFS{
		"index.html":      {Data: []byte("<html>dashboard</html>")},
		"assets/app-1.js": {Data: []byte("console.log(1)")},
		"favicon.svg":     {Data: []byte("<svg/>")},
	}
	e := newEnv(t, func(c *Config) { c.Web = web })

	if rec := e.do(http.MethodGet, "/", ""); rec.Code != http.StatusFound || rec.Header().Get("Location") != "/app/" {
		t.Fatalf("root: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec := e.do(http.MethodGet, "/app/", ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), "dashboard") {
		t.Fatalf("index: %d %s", rec.Code, rec.Body)
	}
	rec := e.do(http.MethodGet, "/app/assets/app-1.js", "")
	if rec.Code != 200 || rec.Body.String() != "console.log(1)" || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("asset: %d %v", rec.Code, rec.Header())
	}
	if rec := e.do(http.MethodGet, "/app/links/abcd", ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), "dashboard") {
		t.Fatalf("spa fallback: %d", rec.Code)
	}

	e = newEnv(t, func(c *Config) { c.Web = fstest.MapFS{".gitkeep": {}} })
	if rec := e.do(http.MethodGet, "/app/", ""); rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "make web-build") {
		t.Fatalf("missing build hint: %d %s", rec.Code, rec.Body)
	}
}

func TestEndToEnd_ClickToStats(t *testing.T) {
	st := memory.New()
	rec := analytics.NewRecorder(st, analytics.RecorderOptions{FlushInterval: time.Hour, Logger: discard})
	svc := shortener.NewService(st, cache.NewMemory(nil), shortener.Options{Logger: discard})
	h := New(Config{Service: svc, Recorder: rec, BaseURL: "http://x", Logger: discard})
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := srv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	resp, err := client.Post(srv.URL+"/api/links", "application/json", bytes.NewBufferString(`{"url":"https://example.com","alias":"e2e-link"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/e2e-link", nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusFound {
			t.Fatalf("redirect %d: %d", i, resp.StatusCode)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rec.Close(ctx); err != nil {
		t.Fatal(err)
	}
	stats, err := st.Stats(ctx, "e2e-link")
	if err != nil || stats.TotalClicks != 3 || stats.ByDevice["desktop"] != 3 || stats.ByCountry["unknown"] != 3 {
		t.Fatalf("stats = %+v, %v", stats, err)
	}
}
