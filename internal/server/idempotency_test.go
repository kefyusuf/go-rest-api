package server_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-lang/internal/auth"
	"go-lang/internal/idempotency"
	"go-lang/internal/model"
	"go-lang/internal/server"
	"go-lang/internal/store"
)

func newIdempotentApp(t *testing.T) (*httptest.Server, *idempotency.MemoryStore) {
	t.Helper()
	store := store.NewMemoryUserStore()
	idempStore := idempotency.NewMemoryStore(5 * time.Minute)
	access, _ := auth.NewTokenIssuer(testJWTSecret, 15*time.Minute, "test", auth.KindAccess)
	refresh, _ := auth.NewTokenIssuer(testJWTSecret, 24*time.Hour, "test", auth.KindRefresh)
	blacklist := auth.NewBlacklist()
	app := server.New(store, newTestLogger(), server.Options{
		BcryptCost:       4,
		IdempotencyStore: idempStore,
		TokenIssuer:      access,
		RefreshIssuer:    refresh,
		Blacklist:        blacklist,
	})
	ts := httptest.NewServer(app)
	return ts, idempStore
}

func TestIdempotencyReplaysSameKeySameBody(t *testing.T) {
	ts, _ := newIdempotentApp(t)
	defer ts.Close()

	body := mustJSON(t, model.CreateUserRequest{
		Name: "Ada", Email: "ada@example.com", Password: "pw",
	})

	post := func() *http.Response {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "users-create-1")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("post: %v", err)
		}
		return res
	}

	first := post()
	defer first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		buf, _ := io.ReadAll(first.Body)
		t.Fatalf("expected 201, got %d: %s", first.StatusCode, string(buf))
	}

	second := post()
	defer second.Body.Close()
	if second.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 on replay, got %d", second.StatusCode)
	}
	if second.Header.Get("Idempotent-Replay") != "true" {
		t.Fatalf("expected Idempotent-Replay header on replay, got %q", second.Header.Get("Idempotent-Replay"))
	}
}

func TestIdempotencyRejectsSameKeyDifferentBody(t *testing.T) {
	ts, _ := newIdempotentApp(t)
	defer ts.Close()

	first := mustJSON(t, model.CreateUserRequest{
		Name: "Ada", Email: "ada@example.com", Password: "pw",
	})
	second := mustJSON(t, model.CreateUserRequest{
		Name: "Grace", Email: "grace@example.com", Password: "pw",
	})

	req1, _ := http.NewRequest(http.MethodPost, ts.URL+"/users", bytes.NewReader(first))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", "key-1")
	res1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	res1.Body.Close()
	if res1.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", res1.StatusCode)
	}

	req2, _ := http.NewRequest(http.MethodPost, ts.URL+"/users", bytes.NewReader(second))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", "key-1")
	res2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", res2.StatusCode)
	}
}

func TestIdempotencyNoKeyAlwaysCallsHandler(t *testing.T) {
	ts, _ := newIdempotentApp(t)
	defer ts.Close()

	body := mustJSON(t, model.CreateUserRequest{
		Name: "User", Email: "user@example.com", Password: "pw",
	})

	first, err := http.Post(ts.URL+"/users", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	defer first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", first.StatusCode)
	}

	second, err := http.Post(ts.URL+"/users", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	defer second.Body.Close()
	if second.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 (duplicate email) without idempotency key, got %d", second.StatusCode)
	}
}

func TestIdempotencyReplaysOnRegister(t *testing.T) {
	ts, _ := newIdempotentApp(t)
	defer ts.Close()

	body := mustJSON(t, model.CreateUserRequest{
		Name: "Ada", Email: "ada@example.com", Password: "pw",
	})

	post := func() *http.Response {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "register-1")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("post: %v", err)
		}
		return res
	}

	first := post()
	defer first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		buf, _ := io.ReadAll(first.Body)
		t.Fatalf("expected 201, got %d: %s", first.StatusCode, string(buf))
	}

	second := post()
	defer second.Body.Close()
	if second.StatusCode != http.StatusCreated {
		t.Fatalf("expected replay 201, got %d", second.StatusCode)
	}
	if second.Header.Get("Idempotent-Replay") != "true" {
		t.Fatalf("expected Idempotent-Replay=true on register replay")
	}
}
