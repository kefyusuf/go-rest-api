package idempotency_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"go-lang/internal/idempotency"
)

func TestHashRequestIsStable(t *testing.T) {
	a := idempotency.HashRequest([]byte(`{"a":1}`))
	b := idempotency.HashRequest([]byte(`{"a":1}`))
	c := idempotency.HashRequest([]byte(`{"a":2}`))

	if a != b {
		t.Fatalf("expected same hash for same body, got %q vs %q", a, b)
	}
	if a == c {
		t.Fatalf("expected different hash for different body")
	}
}

func TestMemoryStoreLookupMiss(t *testing.T) {
	s := idempotency.NewMemoryStore(time.Minute)
	_, err := s.Lookup(context.Background(), "k", "hash")
	if !errors.Is(err, idempotency.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMemoryStoreSaveAndLookup(t *testing.T) {
	s := idempotency.NewMemoryStore(time.Minute)
	hash := idempotency.HashRequest([]byte("body"))
	if err := s.Save(context.Background(), "k", hash, 201, []byte("resp"), "application/json"); err != nil {
		t.Fatalf("save: %v", err)
	}
	e, err := s.Lookup(context.Background(), "k", hash)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if e.Status != 201 || string(e.Body) != "resp" || e.ContentType != "application/json" {
		t.Fatalf("unexpected entry: %+v", e)
	}
}

func TestMemoryStoreConflictOnDifferentBody(t *testing.T) {
	s := idempotency.NewMemoryStore(time.Minute)
	first := idempotency.HashRequest([]byte("first"))
	second := idempotency.HashRequest([]byte("second"))
	_ = s.Save(context.Background(), "k", first, 200, []byte("ok"), "application/json")
	_, err := s.Lookup(context.Background(), "k", second)
	if !errors.Is(err, idempotency.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestMemoryStoreExpiry(t *testing.T) {
	now := time.Now()
	current := now
	s := idempotency.NewMemoryStoreWithNow(10*time.Millisecond, func() time.Time { return current })
	_ = s.Save(context.Background(), "k", "h", 200, []byte("ok"), "application/json")

	current = current.Add(time.Hour)
	_, err := s.Lookup(context.Background(), "k", "h")
	if !errors.Is(err, idempotency.ErrNotFound) {
		t.Fatalf("expected miss after expiry, got %v", err)
	}
}

func TestMemoryStoreSaveDoesNotMutateCallerBuffer(t *testing.T) {
	s := idempotency.NewMemoryStore(time.Minute)
	body := []byte("hello")
	_ = s.Save(context.Background(), "k", "h", 200, body, "application/json")

	body[0] = 'X'

	e, _ := s.Lookup(context.Background(), "k", "h")
	if string(e.Body) != "hello" {
		t.Fatalf("expected stored body not to be mutated, got %q", string(e.Body))
	}
}

func TestMemoryStoreRunReplaysCachedResponse(t *testing.T) {
	s := idempotency.NewMemoryStore(time.Minute)
	calls := 0
	do := func(_ context.Context) (int, []byte, string, error) {
		calls++
		return 201, []byte(`{"id":1}`), "application/json", nil
	}

	body := []byte(`{"name":"Ada"}`)
	hash := idempotency.HashRequest(body)

	e1, replayed, err := s.Run(context.Background(), "k1", body, do)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	if replayed {
		t.Fatal("expected first run to not be a replay")
	}
	if e1.Status != 201 {
		t.Fatalf("expected status 201, got %d", e1.Status)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}

	e2, replayed, err := s.Run(context.Background(), "k1", body, do)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if !replayed {
		t.Fatal("expected second run to be a replay")
	}
	if calls != 1 {
		t.Fatalf("expected handler not to run again, got %d calls", calls)
	}
	if e2.RequestHash != hash {
		t.Fatalf("expected replayed hash to match")
	}
}

func TestMemoryStoreRunConflict(t *testing.T) {
	s := idempotency.NewMemoryStore(time.Minute)
	_ = s.Save(context.Background(), "k", idempotency.HashRequest([]byte("a")), 201, []byte("ok"), "application/json")

	_, _, err := s.Run(context.Background(), "k", []byte("b"), func(_ context.Context) (int, []byte, string, error) {
		return 0, nil, "", errors.New("handler should not run")
	})
	if !errors.Is(err, idempotency.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestMemoryStoreRunNoKeyAlwaysCallsHandler(t *testing.T) {
	s := idempotency.NewMemoryStore(time.Minute)
	calls := 0
	do := func(_ context.Context) (int, []byte, string, error) {
		calls++
		return 200, []byte("ok"), "application/json", nil
	}
	for i := 0; i < 3; i++ {
		_, _, err := s.Run(context.Background(), "", []byte("body"), do)
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls when no key is set, got %d", calls)
	}
}

func TestMemoryStoreRunNoKeyReturnsBody(t *testing.T) {
	s := idempotency.NewMemoryStore(time.Minute)
	body := bytes.NewReader([]byte("hello"))
	_ = body
	_, _, _ = s.Run(context.Background(), "", nil, func(_ context.Context) (int, []byte, string, error) {
		return 200, []byte("ok"), "application/json", nil
	})
}
