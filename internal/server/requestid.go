package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

const (
	requestIDHeader = "X-Request-Id"
	requestIDKey    = "request_id"
	requestIDBytes  = 16
)

type ctxKey int

const requestIDCtxKey ctxKey = 1

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			id = newRequestID()
		}

		w.Header().Set(requestIDHeader, id)

		ctx := context.WithValue(r.Context(), requestIDCtxKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDCtxKey).(string); ok {
		return v
	}
	return ""
}

func newRequestID() string {
	b := make([]byte, requestIDBytes)
	if _, err := rand.Read(b); err != nil {
		return "req-unknown"
	}
	return hex.EncodeToString(b)
}
