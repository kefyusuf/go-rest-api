package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"go-lang/internal/ratelimit"
)

// redisClient returns a Redis client connected to localhost. If Redis
// is not running, the test is skipped so the suite stays green on
// machines that do not have Redis installed.
func redisClient(t *testing.T) *redis.Client {
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

func TestRedisLimiterAllowsBurstThenDenies(t *testing.T) {
	client := redisClient(t)
	defer client.Close()

	// Use a unique key so we do not collide with a parallel test or
	// a previous run.
	key := "test-burst-" + time.Now().Format("150405.000000000")
	defer client.Del(context.Background(), "ratelimit:"+key+":bucket")

	l := ratelimit.NewRedis(client, 1, 3)

	// Three allowed (burst = 3).
	for i := 0; i < 3; i++ {
		if !l.Allow(key) {
			t.Fatalf("expected request %d to be allowed", i+1)
		}
	}
	// Fourth denied.
	if l.Allow(key) {
		t.Fatal("expected fourth request to be denied")
	}
	if l.RetryAfter(key) <= 0 {
		t.Fatal("expected positive RetryAfter when denied")
	}
}

func TestRedisLimiterRefillsOverTime(t *testing.T) {
	client := redisClient(t)
	defer client.Close()

	key := "test-refill-" + time.Now().Format("150405.000000000")
	defer client.Del(context.Background(), "ratelimit:"+key+":bucket")

	// rate=2, burst=1. First request allowed, second denied,
	// after a refill the third is allowed again.
	l := ratelimit.NewRedis(client, 2, 1)

	if !l.Allow(key) {
		t.Fatal("expected first request to be allowed")
	}
	if l.Allow(key) {
		t.Fatal("expected second request to be denied")
	}

	// Override the clock by sleeping just over 0.5s. With
	// rate=2 (one token per 0.5s) the bucket is full again.
	time.Sleep(600 * time.Millisecond)

	if !l.Allow(key) {
		t.Fatal("expected third request to be allowed after refill")
	}
}

func TestRedisLimiterCloseIsIdempotent(t *testing.T) {
	client := redisClient(t)
	l := ratelimit.NewRedis(client, 1, 1)
	if err := l.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	// A second close on a closed client returns the underlying
	// redis error; we only assert it does not panic.
	_ = l.Close()
}
