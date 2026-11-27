package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr string

	BaseURL string

	DatabaseURL string

	RedisURL string

	CacheTTL time.Duration

	RateLimitRPS   float64
	RateLimitBurst int

	TrustProxy bool

	AllowPrivateURLs bool

	DemoSeed bool

	AnalyticsBuffer int

	LogLevel string

	ShutdownTimeout time.Duration
}

func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Addr:             envOr(getenv, "ADDR", ":8080"),
		BaseURL:          strings.TrimRight(envOr(getenv, "BASE_URL", "http://localhost:8080"), "/"),
		DatabaseURL:      getenv("DATABASE_URL"),
		RedisURL:         getenv("REDIS_URL"),
		LogLevel:         strings.ToLower(envOr(getenv, "LOG_LEVEL", "info")),
		TrustProxy:       envBool(getenv, "TRUST_PROXY"),
		AllowPrivateURLs: envBool(getenv, "ALLOW_PRIVATE_URLS"),
		DemoSeed:         envBool(getenv, "DEMO_SEED"),
	}
	if port := getenv("PORT"); port != "" && getenv("ADDR") == "" {
		cfg.Addr = ":" + port
	}

	var err error
	if cfg.CacheTTL, err = envDuration(getenv, "CACHE_TTL", 10*time.Minute); err != nil {
		return cfg, err
	}
	if cfg.ShutdownTimeout, err = envDuration(getenv, "SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		return cfg, err
	}
	if cfg.RateLimitRPS, err = envFloat(getenv, "RATE_LIMIT_RPS", 10); err != nil {
		return cfg, err
	}
	if cfg.RateLimitBurst, err = envInt(getenv, "RATE_LIMIT_BURST", 20); err != nil {
		return cfg, err
	}
	if cfg.AnalyticsBuffer, err = envInt(getenv, "ANALYTICS_BUFFER", 4096); err != nil {
		return cfg, err
	}

	if cfg.RateLimitRPS <= 0 || cfg.RateLimitBurst < 1 {
		return cfg, fmt.Errorf("RATE_LIMIT_RPS must be > 0 and RATE_LIMIT_BURST >= 1")
	}
	if cfg.CacheTTL <= 0 {
		return cfg, fmt.Errorf("CACHE_TTL must be positive")
	}
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return cfg, fmt.Errorf("LOG_LEVEL must be debug, info, warn or error (got %q)", cfg.LogLevel)
	}
	return cfg, nil
}

func envOr(getenv func(string) string, key, def string) string {
	if v := getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(getenv func(string) string, key string) bool {
	switch strings.ToLower(getenv(key)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func envInt(getenv func(string) string, key string, def int) (int, error) {
	v := getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return n, nil
}

func envFloat(getenv func(string) string, key string, def float64) (float64, error) {
	v := getenv(key)
	if v == "" {
		return def, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return f, nil
}

func envDuration(getenv func(string) string, key string, def time.Duration) (time.Duration, error) {
	v := getenv(key)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return d, nil
}
