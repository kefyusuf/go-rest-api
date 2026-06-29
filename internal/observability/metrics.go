// Package observability exposes Prometheus metrics and health probes
// for the API. It is intentionally small: a single registry, two
// custom HTTP metrics, the standard process metrics, and a few
// convenience constructors.
package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	registry *prometheus.Registry

	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	inFlight        prometheus.Gauge

	startedAt time.Time
}

func NewMetrics(serviceName, environment string) *Metrics {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	constLabels := prometheus.Labels{
		"service":     serviceName,
		"environment": environment,
	}

	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "http_requests_total",
			Help:        "Total number of HTTP requests handled, partitioned by method, path, and status.",
			ConstLabels: constLabels,
		},
		[]string{"method", "path", "status"},
	)

	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        "http_request_duration_seconds",
			Help:        "Histogram of HTTP request duration in seconds.",
			ConstLabels: constLabels,
			Buckets:     prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	inFlight := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name:        "http_in_flight_requests",
			Help:        "Number of HTTP requests currently being handled.",
			ConstLabels: constLabels,
		},
	)

	registry.MustRegister(requestsTotal, requestDuration, inFlight)

	return &Metrics{
		registry:        registry,
		requestsTotal:   requestsTotal,
		requestDuration: requestDuration,
		inFlight:        inFlight,
		startedAt:       time.Now(),
	}
}

func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{Registry: m.registry})
}

// RecordHTTP stores the access metrics for a single request. The path
// argument should be a stable path (for example the route pattern),
// not a raw URL with query string or id parameters.
func (m *Metrics) RecordHTTP(method, path string, status int, duration time.Duration) {
	labels := prometheus.Labels{
		"method": method,
		"path":   path,
		"status": strconv.Itoa(status),
	}
	m.requestsTotal.With(labels).Inc()
	m.requestDuration.With(labels).Observe(duration.Seconds())
}

func (m *Metrics) IncInFlight() {
	m.inFlight.Inc()
}

func (m *Metrics) DecInFlight() {
	m.inFlight.Dec()
}

func (m *Metrics) StartedAt() time.Time {
	return m.startedAt
}
