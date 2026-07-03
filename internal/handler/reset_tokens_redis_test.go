package handler_test

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"go-lang/internal/handler"
)

func resetTokenRedisClient(t *testing.T) *redis.Client {
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

func TestRedisResetTokenStoreIssueConsume(t *testing.T) {
	client := resetTokenRedisClient(t)
	defer client.Close()

	store := handler.NewRedisResetTokenStore(client)
	now := time.Now()

	token, expiresAt := store.Issue(42, func() time.Time { return now }, time.Minute)
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if !expiresAt.After(now) {
		t.Fatalf("expiresAt should be in the future")
	}

	// Cleanup the key on exit so we do not leak state across runs.
	defer client.Del(context.Background(), "reset_token:"+token)

	userID, ok := store.Consume(token, func() time.Time { return now })
	if !ok {
		t.Fatal("expected Consume to return ok=true on the first call")
	}
	if userID != 42 {
		t.Fatalf("expected user id 42, got %d", userID)
	}

	// Single-use: a second Consume must return ok=false.
	_, ok = store.Consume(token, func() time.Time { return now })
	if ok {
		t.Fatal("expected Consume to be single-use")
	}
}

func TestRedisResetTokenStoreExpiredIsRejected(t *testing.T) {
	client := resetTokenRedisClient(t)
	defer client.Close()

	store := handler.NewRedisResetTokenStore(client)
	issuedAt := time.Now()
	token, _ := store.Issue(7, func() time.Time { return issuedAt }, 10*time.Millisecond)
	defer client.Del(context.Background(), "reset_token:"+token)

	// Wait past the TTL.
	time.Sleep(50 * time.Millisecond)

	_, ok := store.Consume(token, func() time.Time { return time.Now() })
	if ok {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestRedisResetTokenStoreMissIsRejected(t *testing.T) {
	client := resetTokenRedisClient(t)
	defer client.Close()

	store := handler.NewRedisResetTokenStore(client)

	_, ok := store.Consume("definitely-not-a-real-token", func() time.Time { return time.Now() })
	if ok {
		t.Fatal("expected a missing token to be rejected")
	}
}
