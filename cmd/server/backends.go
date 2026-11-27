package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/portfolio-author/linklens/internal/cache"
	"github.com/portfolio-author/linklens/internal/config"
	"github.com/portfolio-author/linklens/internal/ratelimit"
	"github.com/portfolio-author/linklens/internal/store"
	"github.com/portfolio-author/linklens/internal/store/memory"
	"github.com/portfolio-author/linklens/internal/store/postgres"
)

type backends struct {
	store     store.Store
	cache     cache.Cache
	limiter   ratelimit.Limiter
	storeKind string
	cacheKind string
	closers   []func()
}

func (b *backends) close() {
	for i := len(b.closers) - 1; i >= 0; i-- {
		b.closers[i]()
	}
}

func buildBackends(ctx context.Context, cfg config.Config, logger *slog.Logger) (*backends, error) {
	b := &backends{}
	limitCfg := ratelimit.Config{Rate: cfg.RateLimitRPS, Burst: cfg.RateLimitBurst}
	if err := limitCfg.Validate(); err != nil {
		return nil, err
	}

	connectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if cfg.DatabaseURL != "" {
		pg, err := postgres.Open(connectCtx, cfg.DatabaseURL)
		if err != nil {
			return nil, fmt.Errorf("postgres: %w", err)
		}
		b.closers = append(b.closers, pg.Close)
		if err := pg.Migrate(connectCtx); err != nil {
			b.close()
			return nil, fmt.Errorf("postgres migrate: %w", err)
		}
		b.store, b.storeKind = pg, "postgres"
	} else {
		logger.Warn("DATABASE_URL not set, using in-memory store (data is lost on restart)")
		b.store, b.storeKind = memory.New(), "memory"
	}

	if cfg.RedisURL != "" {
		opts, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			b.close()
			return nil, fmt.Errorf("redis url: %w", err)
		}
		client := redis.NewClient(opts)
		if err := client.Ping(connectCtx).Err(); err != nil {
			_ = client.Close()
			b.close()
			return nil, fmt.Errorf("redis: %w", err)
		}
		b.closers = append(b.closers, func() { _ = client.Close() })
		b.cache, b.cacheKind = cache.NewRedis(client), "redis"
		b.limiter = ratelimit.NewRedis(client, limitCfg, nil)
	} else {
		b.cache, b.cacheKind = cache.NewMemory(nil), "memory"
		b.limiter = ratelimit.NewMemory(limitCfg, nil)
	}
	return b, nil
}
