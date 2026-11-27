package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/portfolio-author/linklens/internal/analytics"
	"github.com/portfolio-author/linklens/internal/cache"
	"github.com/portfolio-author/linklens/internal/shortener"
	"github.com/portfolio-author/linklens/internal/store/memory"
)

type nopRecorder struct{}

func (nopRecorder) Record(analytics.Event) bool { return true }

func BenchmarkRedirect(b *testing.B) {
	st := memory.New()
	svc := shortener.NewService(st, cache.NewMemory(nil), shortener.Options{Logger: discard})
	h := New(Config{Service: svc, Recorder: nopRecorder{}, BaseURL: "http://x", Logger: discard})

	create := httptest.NewRequest(http.MethodPost, "/api/links", strings.NewReader(`{"url":"https://example.com","alias":"bench"}`))
	h.ServeHTTP(httptest.NewRecorder(), create)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/bench", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusFound {
				b.Fatalf("status %d", rec.Code)
			}
		}
	})
}
