package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDUsesIncomingHeader(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := RequestIDFromContext(r.Context())
		if got != "abc-123" {
			t.Fatalf("expected context request id abc-123, got %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(requestIDHeader, "abc-123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get(requestIDHeader); got != "abc-123" {
		t.Fatalf("expected response header to echo incoming request id, got %q", got)
	}
}

func TestRequestIDGeneratesWhenMissing(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := RequestIDFromContext(r.Context())
		if got == "" {
			t.Fatal("expected a generated request id, got empty string")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	header := rec.Header().Get(requestIDHeader)
	if header == "" {
		t.Fatal("expected response header to contain a generated request id")
	}
	if header == "req-unknown" {
		t.Fatal("expected a real generated id, not the fallback")
	}
}

func TestRequestIDFromContextEmptyForPlainContext(t *testing.T) {
	if got := RequestIDFromContext(context.Background()); got != "" {
		t.Fatalf("expected empty id for plain context, got %q", got)
	}
}
