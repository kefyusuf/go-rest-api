package server

import (
	"net/http"
	"time"

	"go-lang/internal/observability"
)

// MetricsMiddleware records http_requests_total,
// http_request_duration_seconds, and http_in_flight_requests for
// every request that passes through it. If m is nil, the middleware
// is a no-op so tests that do not care about metrics can omit it.
func MetricsMiddleware(m *observability.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if m == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			m.IncInFlight()
			defer m.DecInFlight()

			rec := &metricsRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			m.RecordHTTP(r.Method, r.URL.Path, rec.status, time.Since(startedAt))
		})
	}
}

type metricsRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *metricsRecorder) WriteHeader(statusCode int) {
	if !r.wroteHeader {
		r.status = statusCode
		r.wroteHeader = true
		r.ResponseWriter.WriteHeader(statusCode)
	}
}
