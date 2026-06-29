package server

import (
	"net"
	"net/http"
	"strings"

	"go-lang/internal/model"
	"go-lang/internal/ratelimit"
	"go-lang/internal/response"
)

type RateLimitConfig struct {
	Limiter *ratelimit.Limiter
	KeyFunc func(r *http.Request) string
}

func RateLimit(cfg RateLimitConfig) func(http.Handler) http.Handler {
	if cfg.KeyFunc == nil {
		cfg.KeyFunc = ClientIPKey
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.Limiter == nil {
				next.ServeHTTP(w, r)
				return
			}
			key := cfg.KeyFunc(r)
			if !cfg.Limiter.Allow(key) {
				retry := cfg.Limiter.RetryAfter(key)
				if retry > 0 {
					w.Header().Set("Retry-After", retry.String())
				}
				response.Error(w, http.StatusTooManyRequests, model.ErrorCodeRateLimited, "rate limit exceeded", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func ClientIPKey(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			xff = strings.TrimSpace(xff[:i])
		}
		if xff != "" {
			return xff
		}
	}
	if rip := r.Header.Get("X-Real-IP"); rip != "" {
		return rip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
