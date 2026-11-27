package ratelimit

import (
	"encoding/json"
	"log/slog"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
)

type KeyFunc func(*http.Request) string

type MiddlewareOptions struct {
	Key KeyFunc

	Limit int

	OnLimited func()

	Logger *slog.Logger
}

func Middleware(l Limiter, opts MiddlewareOptions) func(http.Handler) http.Handler {
	if opts.Key == nil {
		opts.Key = ClientIP(false)
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			d, err := l.Allow(r.Context(), opts.Key(r))
			if err != nil {

				opts.Logger.Warn("rate limiter unavailable, failing open", "error", err)
				next.ServeHTTP(w, r)
				return
			}
			if opts.Limit > 0 {
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(opts.Limit))
			}
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(d.Remaining))
			if !d.Allowed {
				if opts.OnLimited != nil {
					opts.OnLimited()
				}
				retry := int(math.Ceil(d.RetryAfter.Seconds()))
				if retry < 1 {
					retry = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(retry))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func ClientIP(trustProxy bool) KeyFunc {
	return func(r *http.Request) string {
		if trustProxy {
			if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				if first, _, ok := strings.Cut(xff, ","); ok {
					return strings.TrimSpace(first)
				}
				return strings.TrimSpace(xff)
			}
			if rip := r.Header.Get("X-Real-IP"); rip != "" {
				return strings.TrimSpace(rip)
			}
		}
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return r.RemoteAddr
		}
		return host
	}
}
