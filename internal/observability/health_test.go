package observability_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-lang/internal/observability"
)

type stubPinger struct {
	err error
}

func (s stubPinger) PingContext(ctx context.Context) error { return s.err }

func decodeHealth(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return payload
}

func TestLivenessAlwaysSucceeds(t *testing.T) {
	probes := observability.NewHealthProbes("svc", "1.0.0", "test")
	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rec := httptest.NewRecorder()
	probes.Liveness(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") == "" {
		t.Fatal("expected content-type set")
	}
	if rec.Header().Get("Content-Type")[:16] != "application/json" {
		t.Fatalf("expected JSON content type, got %q", rec.Header().Get("Content-Type"))
	}

	body := decodeHealth(t, rec)
	if body["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", body["status"])
	}
}

func TestLivenessRejectsPost(t *testing.T) {
	probes := observability.NewHealthProbes("svc", "1.0.0", "test")
	req := httptest.NewRequest(http.MethodPost, "/health/live", nil)
	rec := httptest.NewRecorder()
	probes.Liveness(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestReadinessSucceedsWhenDBIsReachable(t *testing.T) {
	probes := observability.NewHealthProbes("svc", "1.0.0", "test")
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	probes.Readiness(stubPinger{})(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestReadinessFailsWhenDBIsDown(t *testing.T) {
	probes := observability.NewHealthProbes("svc", "1.0.0", "test")
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	probes.Readiness(stubPinger{err: errors.New("connection refused")})(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestReadinessSkipsDBWhenPingerIsNil(t *testing.T) {
	probes := observability.NewHealthProbes("svc", "1.0.0", "test")
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	probes.Readiness(nil)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when no pinger is set, got %d", rec.Code)
	}
}
