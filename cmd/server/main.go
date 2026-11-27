package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/portfolio-author/linklens/internal/analytics"
	"github.com/portfolio-author/linklens/internal/api"
	"github.com/portfolio-author/linklens/internal/config"
	"github.com/portfolio-author/linklens/internal/demo"
	"github.com/portfolio-author/linklens/internal/metrics"
	"github.com/portfolio-author/linklens/internal/shortener"
	"github.com/portfolio-author/linklens/web"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv, os.Stdout, nil); err != nil {
		fmt.Fprintln(os.Stderr, "linklens:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, getenv func(string) string, logOut io.Writer, ready chan<- string) error {
	cfg, err := config.Load(getenv)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	logger := newLogger(logOut, cfg.LogLevel)
	slog.SetDefault(logger)

	deps, err := buildBackends(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer deps.close()

	if cfg.DemoSeed {
		if deps.storeKind != "memory" {
			logger.Warn("DEMO_SEED ignored: demo data is only loaded into the in-memory store")
		} else {
			n, err := demo.Seed(ctx, deps.store, time.Now())
			if err != nil {
				return fmt.Errorf("demo seed: %w", err)
			}
			logger.Info("demo data loaded", "clicks", n)
		}
	}

	m := metrics.New()
	svc := shortener.NewService(deps.store, deps.cache, shortener.Options{
		Validator: shortener.URLValidator{AllowPrivate: cfg.AllowPrivateURLs},
		CacheTTL:  cfg.CacheTTL,
		Observer:  m,
		Logger:    logger,
	})
	recorder := analytics.NewRecorder(deps.store, analytics.RecorderOptions{
		BufferSize: cfg.AnalyticsBuffer,
		Observer:   m,
		Logger:     logger,
	})

	handler := api.New(api.Config{
		Service:        svc,
		Recorder:       recorder,
		BaseURL:        cfg.BaseURL,
		Limiter:        deps.limiter,
		RateLimitBurst: cfg.RateLimitBurst,
		TrustProxy:     cfg.TrustProxy,
		Metrics:        m,
		MetricsHandler: m.Handler(),
		HealthChecks:   map[string]api.Pinger{"store": deps.store, "cache": deps.cache},
		Web:            web.Dist(),
		Logger:         logger,
	})

	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.Addr, err)
	}
	logger.Info("linklens listening", "addr", ln.Addr().String(), "base_url", cfg.BaseURL,
		"store", deps.storeKind, "cache", deps.cacheKind)
	if ready != nil {
		ready <- ln.Addr().String()
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		logger.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown", "error", err)
	}

	if err := recorder.Close(shutdownCtx); err != nil {
		logger.Error("analytics flush", "error", err, "dropped", recorder.Dropped())
	}
	return nil
}

func newLogger(w io.Writer, level string) *slog.Logger {
	var lv slog.Level
	_ = lv.UnmarshalText([]byte(level))
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lv}))
}
