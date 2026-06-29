package server

import (
	"log/slog"
	"net/http"
	"time"
)

type AccessLogEntry struct {
	Method     string        `json:"method"`
	Path       string        `json:"path"`
	Status     int           `json:"status"`
	DurationMS int64         `json:"duration_ms"`
	RequestID  string        `json:"requestId,omitempty"`
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func newStatusRecorder(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func AccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			recorder := newStatusRecorder(w)

			next.ServeHTTP(recorder, r)

			logger.LogAttrs(r.Context(), slog.LevelInfo, "http_request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", recorder.statusCode),
				slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
				slog.String("request_id", RequestIDFromContext(r.Context())),
			)
		})
	}
}
