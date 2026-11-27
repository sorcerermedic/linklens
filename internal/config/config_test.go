package config

import (
	"testing"
	"time"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":8080" || cfg.BaseURL != "http://localhost:8080" || cfg.CacheTTL != 10*time.Minute ||
		cfg.RateLimitRPS != 10 || cfg.RateLimitBurst != 20 || cfg.AnalyticsBuffer != 4096 || cfg.LogLevel != "info" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.DatabaseURL != "" || cfg.RedisURL != "" || cfg.TrustProxy || cfg.AllowPrivateURLs || cfg.DemoSeed {
		t.Fatalf("optional settings must default to off: %+v", cfg)
	}
}

func TestLoad_Values(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"PORT":               "9090",
		"BASE_URL":           "https://lnk.example/",
		"DATABASE_URL":       "postgres://x",
		"REDIS_URL":          "redis://y",
		"CACHE_TTL":          "30s",
		"RATE_LIMIT_RPS":     "2.5",
		"RATE_LIMIT_BURST":   "5",
		"TRUST_PROXY":        "true",
		"ALLOW_PRIVATE_URLS": "1",
		"ANALYTICS_BUFFER":   "16",
		"LOG_LEVEL":          "DEBUG",
		"SHUTDOWN_TIMEOUT":   "3s",
		"DEMO_SEED":          "yes",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":9090" || cfg.BaseURL != "https://lnk.example" || cfg.CacheTTL != 30*time.Second ||
		cfg.RateLimitRPS != 2.5 || cfg.RateLimitBurst != 5 || !cfg.TrustProxy || !cfg.AllowPrivateURLs ||
		cfg.AnalyticsBuffer != 16 || cfg.LogLevel != "debug" || cfg.ShutdownTimeout != 3*time.Second || !cfg.DemoSeed {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoad_AddrWinsOverPort(t *testing.T) {
	cfg, err := Load(env(map[string]string{"PORT": "1", "ADDR": "127.0.0.1:2"}))
	if err != nil || cfg.Addr != "127.0.0.1:2" {
		t.Fatalf("cfg=%+v err=%v", cfg, err)
	}
}

func TestLoad_Invalid(t *testing.T) {
	tests := []map[string]string{
		{"CACHE_TTL": "soon"},
		{"CACHE_TTL": "-1s"},
		{"RATE_LIMIT_RPS": "abc"},
		{"RATE_LIMIT_RPS": "0"},
		{"RATE_LIMIT_BURST": "0"},
		{"RATE_LIMIT_BURST": "x"},
		{"ANALYTICS_BUFFER": "1.5"},
		{"LOG_LEVEL": "loud"},
		{"SHUTDOWN_TIMEOUT": "nope"},
	}
	for _, m := range tests {
		if _, err := Load(env(m)); err == nil {
			t.Errorf("expected error for %v", m)
		}
	}
}
