package server

import (
	"net/http"
	"strings"
)

type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	ExposedHeaders []string
	MaxAge         int
}

func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	allowedOrigins := make(map[string]struct{}, len(cfg.AllowedOrigins))
	allowAll := false
	for _, o := range cfg.AllowedOrigins {
		if o == "*" {
			allowAll = true
		}
		allowedOrigins[o] = struct{}{}
	}
	methods := strings.Join(defaultIfEmpty(cfg.AllowedMethods, []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}), ", ")
	headers := strings.Join(defaultIfEmpty(cfg.AllowedHeaders, []string{"Authorization", "Content-Type", "X-Request-Id"}), ", ")
	maxAge := cfg.MaxAge
	if maxAge <= 0 {
		maxAge = 600
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				_, ok := allowedOrigins[origin]
				if allowAll || ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					if !allowAll {
						w.Header().Set("Vary", "Origin")
					}
				}
			}
			if len(cfg.ExposedHeaders) > 0 {
				w.Header().Set("Access-Control-Expose-Headers", strings.Join(cfg.ExposedHeaders, ", "))
			}
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", methods)
				w.Header().Set("Access-Control-Allow-Headers", headers)
				w.Header().Set("Access-Control-Max-Age", maxAgeStr(maxAge))
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func defaultIfEmpty(values []string, fallback []string) []string {
	if len(values) == 0 {
		return fallback
	}
	return values
}

func maxAgeStr(seconds int) string {
	const digits = "0123456789"
	if seconds == 0 {
		return "0"
	}
	buf := make([]byte, 0, 8)
	for seconds > 0 {
		buf = append([]byte{digits[seconds%10]}, buf...)
		seconds /= 10
	}
	return string(buf)
}
