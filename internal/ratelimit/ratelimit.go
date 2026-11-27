package ratelimit

import (
	"context"
	"errors"
	"time"
)

type Config struct {
	Rate float64

	Burst int
}

func (c Config) Validate() error {
	if c.Rate <= 0 {
		return errors.New("ratelimit: rate must be positive")
	}
	if c.Burst < 1 {
		return errors.New("ratelimit: burst must be at least 1")
	}
	return nil
}

type Decision struct {
	Allowed bool

	Remaining int

	RetryAfter time.Duration
}

type Limiter interface {
	Allow(ctx context.Context, key string) (Decision, error)
}

type Clock func() time.Time

func take(tokens float64, last, now time.Time, cfg Config) (float64, Decision) {
	elapsed := now.Sub(last).Seconds()
	if elapsed > 0 {
		tokens += elapsed * cfg.Rate
	}
	if tokens > float64(cfg.Burst) {
		tokens = float64(cfg.Burst)
	}
	if tokens >= 1 {
		tokens--
		return tokens, Decision{Allowed: true, Remaining: int(tokens)}
	}
	wait := time.Duration((1 - tokens) / cfg.Rate * float64(time.Second))
	return tokens, Decision{Allowed: false, Remaining: 0, RetryAfter: wait}
}
