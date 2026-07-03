package jobs_test

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"go-lang/internal/jobs"
)

func deadLetterRedisClient(t *testing.T) *redis.Client {
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

func TestRedisDeadLetterAddAndList(t *testing.T) {
	client := deadLetterRedisClient(t)
	defer client.Close()

	// Wipe the list so each test starts clean. The list key is
	// shared across the suite, so leftover entries from a prior
	// run would confuse the count assertion.
	ctx := context.Background()
	if err := client.Del(ctx, "go-rest-api.jobs.dlq").Err(); err != nil {
		t.Fatalf("wipe dlq: %v", err)
	}
	defer client.Del(ctx, "go-rest-api.jobs.dlq")

	dl := jobs.NewRedisDeadLetter(client)

	now := time.Now()
	dl.Add(jobs.Job{ID: "j-1", Type: "welcome_email", EnqueuedAt: now}, "boom-1")
	dl.Add(jobs.Job{ID: "j-2", Type: "send_sms", EnqueuedAt: now}, "boom-2")

	entries := dl.List()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Job.ID != "j-1" || entries[0].LastErr != "boom-1" {
		t.Fatalf("first entry mismatch: %+v", entries[0])
	}
	if entries[1].Job.ID != "j-2" || entries[1].LastErr != "boom-2" {
		t.Fatalf("second entry mismatch: %+v", entries[1])
	}
}

func TestRedisDeadLetterEmptyList(t *testing.T) {
	client := deadLetterRedisClient(t)
	defer client.Close()

	ctx := context.Background()
	if err := client.Del(ctx, "go-rest-api.jobs.dlq").Err(); err != nil {
		t.Fatalf("wipe dlq: %v", err)
	}
	defer client.Del(ctx, "go-rest-api.jobs.dlq")

	dl := jobs.NewRedisDeadLetter(client)
	entries := dl.List()
	if len(entries) != 0 {
		t.Fatalf("expected empty list, got %d entries", len(entries))
	}
}

func TestRedisDeadLetterSkipCorruptEntry(t *testing.T) {
	client := deadLetterRedisClient(t)
	defer client.Close()

	ctx := context.Background()
	key := "go-rest-api.jobs.dlq"
	if err := client.Del(ctx, key).Err(); err != nil {
		t.Fatalf("wipe dlq: %v", err)
	}
	defer client.Del(ctx, key)

	// Push a corrupt entry alongside a valid one. The valid
	// entry should still come through; the corrupt one is
	// silently skipped.
	if err := client.RPush(ctx, key, "not-json").Err(); err != nil {
		t.Fatalf("rpush corrupt: %v", err)
	}

	dl := jobs.NewRedisDeadLetter(client)
	dl.Add(jobs.Job{ID: "j-good", Type: "good", EnqueuedAt: time.Now()}, "boom-good")

	entries := dl.List()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (corrupt skipped), got %d", len(entries))
	}
	if entries[0].Job.ID != "j-good" {
		t.Fatalf("expected the valid entry, got %+v", entries[0])
	}
}
