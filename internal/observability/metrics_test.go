package observability_test

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go-lang/internal/observability"
)

func TestNewMetricsRegistersCoreMetrics(t *testing.T) {
	m := observability.NewMetrics("test-service", "test")
	if m.Registry() == nil {
		t.Fatal("expected non-nil registry")
	}
	if m.StartedAt().IsZero() {
		t.Fatal("expected a non-zero startedAt")
	}
}

func TestMetricsHandlerExposesPrometheus(t *testing.T) {
	m := observability.NewMetrics("test-service", "test")
	m.IncInFlight()
	m.IncInFlight()
	m.DecInFlight()
	m.RecordHTTP("GET", "/health", 200, 5*time.Millisecond)
	m.RecordHTTP("POST", "/users", 201, 12*time.Millisecond)
	m.RecordHTTP("POST", "/users", 500, 100*time.Millisecond)

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	if !strings.Contains(body, `http_requests_total`) {
		t.Fatalf("expected body to contain http_requests_total, got:\n%s", body)
	}
	if !strings.Contains(body, `path="/health"`) {
		t.Fatalf("expected body to contain path=/health, got:\n%s", body)
	}
	if !strings.Contains(body, `path="/users"`) {
		t.Fatalf("expected body to contain path=/users, got:\n%s", body)
	}
	if !strings.Contains(body, `status="200"`) {
		t.Fatalf("expected body to contain status=200, got:\n%s", body)
	}
	if !strings.Contains(body, `status="500"`) {
		t.Fatalf("expected body to contain status=500, got:\n%s", body)
	}
	if !strings.Contains(body, `http_request_duration_seconds_bucket`) {
		t.Fatalf("expected body to contain http_request_duration_seconds_bucket, got:\n%s", body)
	}
	if !strings.Contains(body, `http_in_flight_requests`) {
		t.Fatalf("expected body to contain http_in_flight_requests, got:\n%s", body)
	}
	if !strings.Contains(body, `go_goroutines`) {
		t.Fatalf("expected body to contain go_goroutines, got:\n%s", body)
	}
	if !strings.Contains(body, `process_cpu_seconds_total`) {
		t.Fatalf("expected body to contain process_cpu_seconds_total, got:\n%s", body)
	}
}
