package jobs_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"go-lang/internal/jobs"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestMemoryQueueEnqueueAndDequeue(t *testing.T) {
	q := jobs.NewMemoryQueue()
	ctx := context.Background()

	job := jobs.Job{ID: "j1", Type: "email", Payload: []byte("hello")}
	if err := q.Enqueue(ctx, job); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if got := q.Len(); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}

	got, ok, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if !ok {
		t.Fatal("expected a job")
	}
	if got.ID != "j1" {
		t.Fatalf("expected j1, got %q", got.ID)
	}
}

func TestMemoryQueueCloseRejectsEnqueue(t *testing.T) {
	q := jobs.NewMemoryQueue()
	_ = q.Close()
	if err := q.Enqueue(context.Background(), jobs.Job{ID: "j1"}); !errors.Is(err, jobs.ErrQueueClosed) {
		t.Fatalf("expected ErrQueueClosed, got %v", err)
	}
}

func TestMemoryQueueCloseUnblocksDequeue(t *testing.T) {
	q := jobs.NewMemoryQueue()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		_, _, _ = q.Dequeue(ctx)
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
	_ = q.Close()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("dequeue did not unblock on close")
	}
}

func TestRegistryCallsHandler(t *testing.T) {
	q := jobs.NewMemoryQueue()
	dead := jobs.NewMemoryDeadLetter()
	reg := jobs.NewRegistry(q, dead, newTestLogger())
	reg.Register("email", jobs.HandlerFunc(func(_ context.Context, j jobs.Job) error {
		if j.Type != "email" {
			t.Fatalf("expected email, got %q", j.Type)
		}
		return nil
	}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reg.Start(ctx, 1)

	if err := reg.Enqueue(ctx, "email", []byte("payload")); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if q.Len() == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	reg.Stop()
	if q.Len() != 0 {
		t.Fatalf("expected queue drained, got %d", q.Len())
	}
}

func TestRegistryRetriesOnError(t *testing.T) {
	q := jobs.NewMemoryQueue()
	dead := jobs.NewMemoryDeadLetter()
	reg := jobs.NewRegistry(q, dead, newTestLogger())

	var calls atomic.Int32
	reg.Register("email", jobs.HandlerFunc(func(_ context.Context, _ jobs.Job) error {
		n := calls.Add(1)
		if n < 3 {
			return errors.New("transient")
		}
		return nil
	}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reg.Start(ctx, 1)

	if err := reg.Enqueue(ctx, "email", []byte("p")); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if calls.Load() >= 3 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	reg.Stop()

	if calls.Load() < 3 {
		t.Fatalf("expected at least 3 attempts, got %d", calls.Load())
	}
}

func TestRegistrySendsToDeadLetterAfterMaxRetries(t *testing.T) {
	q := jobs.NewMemoryQueue()
	dead := jobs.NewMemoryDeadLetter()
	reg := jobs.NewRegistry(q, dead, newTestLogger())

	var calls atomic.Int32
	reg.Register("email", jobs.HandlerFunc(func(_ context.Context, j jobs.Job) error {
		calls.Add(1)
		return errors.New("permanent")
	}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reg.Start(ctx, 1)

	if err := reg.Enqueue(ctx, "email", []byte("p")); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if len(dead.List()) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	reg.Stop()

	if len(dead.List()) != 1 {
		t.Fatalf("expected 1 dead letter, got %d", len(dead.List()))
	}
	if calls.Load() < 2 {
		t.Fatalf("expected at least 2 attempts, got %d", calls.Load())
	}
}

func TestRegistryRetriesUntilMaxRetries(t *testing.T) {
	q := jobs.NewMemoryQueue()
	dead := jobs.NewMemoryDeadLetter()
	reg := jobs.NewRegistry(q, dead, newTestLogger())

	var calls atomic.Int32
	reg.Register("email", jobs.HandlerFunc(func(_ context.Context, _ jobs.Job) error {
		calls.Add(1)
		return errors.New("always")
	}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reg.Start(ctx, 1)

	if err := reg.Enqueue(ctx, "email", []byte("p")); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if len(dead.List()) >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	reg.Stop()

	if calls.Load() < 1 {
		t.Fatalf("expected at least 1 call, got %d", calls.Load())
	}
	if len(dead.List()) == 0 {
		t.Fatalf("expected dead letter entry")
	}
}

func TestRegistryUnknownJobTypeGoesToDeadLetter(t *testing.T) {
	q := jobs.NewMemoryQueue()
	dead := jobs.NewMemoryDeadLetter()
	reg := jobs.NewRegistry(q, dead, newTestLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reg.Start(ctx, 1)

	if err := reg.Enqueue(ctx, "unknown", []byte("p")); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if len(dead.List()) >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	reg.Stop()

	if len(dead.List()) == 0 {
		t.Fatal("expected unknown job type to end up in dead letter")
	}
}

func TestMemoryDeadLetterList(t *testing.T) {
	d := jobs.NewMemoryDeadLetter()
	d.Add(jobs.Job{ID: "a", Type: "email"}, "err1")
	d.Add(jobs.Job{ID: "b", Type: "email"}, "err2")

	entries := d.List()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}
