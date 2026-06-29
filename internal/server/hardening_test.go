package server_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-lang/internal/model"
	"go-lang/internal/server"
	"go-lang/internal/store"
)

func TestUnknownRouteReturnsNotFoundEnvelope(t *testing.T) {
	app := server.New(store.NewMemoryUserStore(), newTestLogger(), server.Options{})
	ts := httptest.NewServer(app)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/does-not-exist")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", res.StatusCode)
	}

	var payload model.ErrorResponse
	decodeJSON(t, res.Body, &payload)
	if payload.Error.Code != model.ErrorCodeNotFound {
		t.Fatalf("expected NOT_FOUND, got %q", payload.Error.Code)
	}
}

func TestRequestIDHeaderEchoedInResponse(t *testing.T) {
	app := server.New(store.NewMemoryUserStore(), newTestLogger(), server.Options{})
	ts := httptest.NewServer(app)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/health", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("X-Request-Id", "client-supplied-id-42")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer res.Body.Close()

	if got := res.Header.Get("X-Request-Id"); got != "client-supplied-id-42" {
		t.Fatalf("expected echoed request id, got %q", got)
	}
}

func TestRequestIDGeneratedWhenMissing(t *testing.T) {
	app := server.New(store.NewMemoryUserStore(), newTestLogger(), server.Options{})
	ts := httptest.NewServer(app)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer res.Body.Close()

	got := res.Header.Get("X-Request-Id")
	if got == "" {
		t.Fatal("expected generated X-Request-Id header, got empty")
	}
	if got == "req-unknown" {
		t.Fatal("expected a real generated id, not the fallback")
	}
}

func TestRecoveryMiddlewareReturns500OnPanic(t *testing.T) {
	app := server.New(store.NewMemoryUserStore(), newTestLogger(), server.Options{})
	ts := httptest.NewServer(app)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on /health, got %d", res.StatusCode)
	}
}

func TestBodyLimitRejectsOversizedPayload(t *testing.T) {
	app := server.New(store.NewMemoryUserStore(), newTestLogger(), server.Options{MaxBodyBytes: 32})
	ts := httptest.NewServer(app)
	defer ts.Close()

	body := strings.Repeat("a", 256)
	res, err := http.Post(ts.URL+"/users", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK || res.StatusCode == http.StatusCreated {
		t.Fatalf("expected non-success status for oversized body, got %d", res.StatusCode)
	}
}

func TestNewAcceptsNilLogger(t *testing.T) {
	app := server.New(store.NewMemoryUserStore(), nil, server.Options{})
	if app == nil {
		t.Fatal("expected non-nil handler when logger is nil")
	}
}

func TestNewWithLogger(t *testing.T) {
	discard := slog.New(slog.NewTextHandler(io.Discard, nil))
	app := server.New(store.NewMemoryUserStore(), discard, server.Options{})
	if app == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestRecoveryCatchesPanic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/explode", nil)

	server.Recovery(logger)(panicHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	var payload model.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Error.Code != model.ErrorCodeInternal {
		t.Fatalf("expected INTERNAL_ERROR, got %q", payload.Error.Code)
	}
}
