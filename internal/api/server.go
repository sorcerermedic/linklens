package api

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/portfolio-author/linklens/internal/analytics"
	"github.com/portfolio-author/linklens/internal/ratelimit"
	"github.com/portfolio-author/linklens/internal/shortener"
)

const maxBodyBytes = 16 << 10

type ClickRecorder interface {
	Record(ev analytics.Event) bool
}

type Pinger interface {
	Ping(ctx context.Context) error
}

type MetricsSink interface {
	ObserveRequest(method, route string, status int, d time.Duration)
	Redirect()
	RateLimited()
}

type Config struct {
	Service  *shortener.Service
	Recorder ClickRecorder

	BaseURL string

	Limiter ratelimit.Limiter

	RateLimitBurst int

	TrustProxy bool

	Metrics        MetricsSink
	MetricsHandler http.Handler

	HealthChecks map[string]Pinger

	Web    fs.FS
	Clock  func() time.Time
	Logger *slog.Logger
}

type Server struct {
	cfg Config
}

func New(cfg Config) http.Handler {
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Metrics == nil {
		cfg.Metrics = noopMetrics{}
	}
	s := &Server{cfg: cfg}
	return s.routes()
}

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(s.instrument)

	r.Get("/healthz", s.handleHealth)
	if s.cfg.MetricsHandler != nil {
		r.Method(http.MethodGet, "/metrics", s.cfg.MetricsHandler)
	}
	if s.cfg.Web != nil {
		spa := spaHandler(s.cfg.Web)
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/app/", http.StatusFound)
		})
		r.Get("/app", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/app/", http.StatusMovedPermanently)
		})
		r.Handle("/app/*", http.StripPrefix("/app", spa))
	}

	r.Group(func(r chi.Router) {
		if s.cfg.Limiter != nil {
			r.Use(ratelimit.Middleware(s.cfg.Limiter, ratelimit.MiddlewareOptions{
				Key:       ratelimit.ClientIP(s.cfg.TrustProxy),
				Limit:     s.cfg.RateLimitBurst,
				OnLimited: s.cfg.Metrics.RateLimited,
				Logger:    s.cfg.Logger,
			}))
		}
		r.Route("/api", func(r chi.Router) {
			r.Post("/links", s.handleCreate)
			r.Get("/links", s.handleList)
			r.Get("/links/{code}/stats", s.handleStats)
			r.Delete("/links/{code}", s.handleDelete)
			r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
				writeError(w, http.StatusNotFound, "not found")
			})
		})
		r.Get("/{code}", s.handleRedirect)
	})
	return r
}

func (s *Server) instrument(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		route := "unmatched"
		if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
			route = rc.RoutePattern()
		}
		status := ww.Status()
		if status == 0 {
			status = http.StatusOK
		}
		elapsed := time.Since(start)
		s.cfg.Metrics.ObserveRequest(r.Method, route, status, elapsed)
		s.cfg.Logger.Debug("request",
			"method", r.Method, "path", r.URL.Path, "route", route,
			"status", status, "duration", elapsed, "request_id", middleware.GetReqID(r.Context()))
	})
}

type noopMetrics struct{}

func (noopMetrics) ObserveRequest(string, string, int, time.Duration) {}
func (noopMetrics) Redirect()                                         {}
func (noopMetrics) RateLimited()                                      {}
