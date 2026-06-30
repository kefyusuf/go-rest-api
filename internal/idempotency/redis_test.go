package idempotency_test

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"go-lang/internal/idempotency"
)

func idempotencyRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	c := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:6379",
		DialTimeout:  200 * time.Millisecond,
		ReadTimeout:  200 * time.Millisecond,
		WriteTimeout: 200 * time.Millisecond,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		t.Skipf("redis not reachable, skipping: %v", err)
	}
	return c
}

func TestRedisStoreRoundTrip(t *testing.T) {
	client := idempotencyRedisClient(t)
	defer client.Close()

	key := "test-roundtrip-" + time.Now().Format("150405.000000000")
	defer client.Del(context.Background(), "idempotency:"+key)

	store := idempotency.NewRedisStore(client, time.Minute)

	hash := idempotency.HashRequest([]byte(`{"name":"Ada"}`))

	e, err := store.Lookup(context.Background(), key, hash)
	if err == nil {
		t.Fatalf("expected miss, got entry %+v", e)
	}
	if err != idempotency.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	if err := store.Save(context.Background(), key, hash, 201, []byte(`{"id":1}`), "application/json"); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := store.Lookup(context.Background(), key, hash)
	if err != nil {
		t.Fatalf("expected hit, got %v", err)
	}
	if got.Status != 201 {
		t.Fatalf("expected status 201, got %d", got.Status)
	}
	if string(got.Body) != `{"id":1}` {
		t.Fatalf("body mismatch: %q", string(got.Body))
	}
	if got.ContentType != "application/json" {
		t.Fatalf("content-type mismatch: %q", got.ContentType)
	}
}

func TestRedisStoreRejectsHashMismatch(t *testing.T) {
	client := idempotencyRedisClient(t)
	defer client.Close()

	key := "test-conflict-" + time.Now().Format("150405.000000000")
	defer client.Del(context.Background(), "idempotency:"+key)

	store := idempotency.NewRedisStore(client, time.Minute)
	first := idempotency.HashRequest([]byte(`{"name":"Ada"}`))
	second := idempotency.HashRequest([]byte(`{"name":"Bob"}`))

	if err := store.Save(context.Background(), key, first, 201, []byte(`{"id":1}`), "application/json"); err != nil {
		t.Fatalf("save: %v", err)
	}

	_, err := store.Lookup(context.Background(), key, second)
	if err != idempotency.ErrConflict {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestRedisStoreTTLExpire(t *testing.T) {
	client := idempotencyRedisClient(t)
	defer client.Close()

	key := "test-ttl-" + time.Now().Format("150405.000000000")
	defer client.Del(context.Background(), "idempotency:"+key)

	// Tiny TTL so the test does not have to wait long.
	store := idempotency.NewRedisStore(client, 250*time.Millisecond)
	hash := idempotency.HashRequest([]byte(`{"a":1}`))

	if err := store.Save(context.Background(), key, hash, 200, []byte("ok"), "text/plain"); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Hit the cache while it is still warm.
	if _, err := store.Lookup(context.Background(), key, hash); err != nil {
		t.Fatalf("expected immediate hit, got %v", err)
	}

	// Wait past the TTL. Redis's minimum EXPIRE TTL is 1s, so the
	// tiny 250ms input gets truncated. Sleep just over a second
	// to be safe across a slow test machine.
	time.Sleep(1100 * time.Millisecond)

	// Lookups after expiry must be a clean miss.
	_, err := store.Lookup(context.Background(), key, hash)
	if err != idempotency.ErrNotFound {
		t.Fatalf("expected ErrNotFound after TTL, got %v", err)
	}
}
