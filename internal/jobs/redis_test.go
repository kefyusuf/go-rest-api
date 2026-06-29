package jobs_test

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"go-lang/internal/jobs"
)

func jobsRedisClient(t *testing.T) *redis.Client {
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

func TestRedisQueueEnqueueDequeue(t *testing.T) {
	client := jobsRedisClient(t)
	defer client.Close()

	stream := "test-queue-" + time.Now().Format("150405.000000000")
	defer client.Del(context.Background(), stream)

	q, err := jobs.NewRedisQueue(client, jobs.RedisConfig{
		Stream:   stream,
		Group:    "test-group",
		Consumer: "test-consumer",
	})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}

	ctx := context.Background()
	job := jobs.Job{
		ID:         "j1",
		Type:       "welcome_email",
		Payload:    []byte(`{"email":"ada@example.com"}`),
		MaxRetries: 3,
		EnqueuedAt: time.Now(),
		RunAfter:   time.Now(),
	}
	if err := q.Enqueue(ctx, job); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	got, ok, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if !ok {
		t.Fatal("expected a job")
	}
	if got.Type != "welcome_email" {
		t.Fatalf("expected type welcome_email, got %q", got.Type)
	}
	if string(got.Payload) != `{"email":"ada@example.com"}` {
		t.Fatalf("payload mismatch: %q", string(got.Payload))
	}

	if err := q.Ack(ctx, got); err != nil {
		t.Fatalf("ack: %v", err)
	}
}

func TestRedisQueueNackReschedules(t *testing.T) {
	client := jobsRedisClient(t)
	defer client.Close()

	stream := "test-nack-" + time.Now().Format("150405.000000000")
	defer client.Del(context.Background(), stream)

	q, err := jobs.NewRedisQueue(client, jobs.RedisConfig{
		Stream:   stream,
		Group:    "test-group",
		Consumer: "test-consumer",
	})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}

	ctx := context.Background()
	job := jobs.Job{
		ID:         "j2",
		Type:       "send_sms",
		Payload:    []byte(`{"to":"+90"}`),
		MaxRetries: 3,
		EnqueuedAt: time.Now(),
		RunAfter:   time.Now(),
	}
	if err := q.Enqueue(ctx, job); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	got, ok, err := q.Dequeue(ctx)
	if err != nil || !ok {
		t.Fatalf("dequeue: ok=%v err=%v", ok, err)
	}

	if err := q.Nack(ctx, got, errTest); err != nil {
		t.Fatalf("nack: %v", err)
	}

	// The retried job is a new stream entry. Dequeue should return
	// it again. The RunAfter is in the future but the stream
	// adapter reads the entry immediately; for at-least-once
	// semantics the worker would compare RunAfter before
	// processing, which is out of scope here.
	_, ok, err = q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("second dequeue: %v", err)
	}
	if !ok {
		t.Fatal("expected the retried job to be re-delivered")
	}
}

func TestRedisQueueLen(t *testing.T) {
	client := jobsRedisClient(t)
	defer client.Close()

	stream := "test-len-" + time.Now().Format("150405.000000000")
	defer client.Del(context.Background(), stream)

	q, err := jobs.NewRedisQueue(client, jobs.RedisConfig{
		Stream:   stream,
		Group:    "test-group",
		Consumer: "test-consumer",
	})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}

	if l := q.Len(); l != 0 {
		t.Fatalf("expected empty queue, got len %d", l)
	}

	for i := 0; i < 3; i++ {
		if err := q.Enqueue(context.Background(), jobs.Job{
			ID:         "j" + string(rune('a'+i)),
			Type:       "t",
			EnqueuedAt: time.Now(),
			RunAfter:   time.Now(),
		}); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}

	if l := q.Len(); l != 3 {
		t.Fatalf("expected len 3, got %d", l)
	}
}

var errTest = redisErrTest{}

type redisErrTest struct{}

func (redisErrTest) Error() string { return "transient test error" }
