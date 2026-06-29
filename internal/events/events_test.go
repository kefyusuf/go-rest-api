package events_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"go-lang/internal/events"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestOutboxEnqueueDequeue(t *testing.T) {
	o := events.NewOutbox()
	ctx := context.Background()

	e := events.Event{Type: "user.created", Topic: "users", Payload: []byte(`{"id":1}`)}
	if err := o.Enqueue(ctx, e); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if o.Len() != 1 {
		t.Fatalf("expected 1, got %d", o.Len())
	}

	got, ok, err := o.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if !ok {
		t.Fatal("expected ok")
	}
	if got.Type != "user.created" {
		t.Fatalf("expected user.created, got %q", got.Type)
	}
	if got.ID == "" {
		t.Fatal("expected id to be set")
	}
}

func TestOutboxCloseUnblocksDequeue(t *testing.T) {
	o := events.NewOutbox()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		_, _, _ = o.Dequeue(ctx)
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
	_ = o.Close()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("dequeue did not unblock on close")
	}
}

func TestOutboxCloseRejectsEnqueue(t *testing.T) {
	o := events.NewOutbox()
	_ = o.Close()
	if err := o.Enqueue(context.Background(), events.Event{Type: "x"}); !errors.Is(err, events.ErrOutboxClosed) {
		t.Fatalf("expected ErrOutboxClosed, got %v", err)
	}
}

func TestOutboxEnqueueSetsDefaults(t *testing.T) {
	o := events.NewOutbox()
	if err := o.Enqueue(context.Background(), events.Event{Type: "x"}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	got, _, _ := o.Dequeue(context.Background())
	if got.ID == "" {
		t.Fatal("expected id to be set")
	}
	if got.OccurredAt.IsZero() {
		t.Fatal("expected OccurredAt to be set")
	}
}

func TestLoggingPublisherDoesNotError(t *testing.T) {
	p := events.NewLoggingPublisher(newTestLogger())
	defer p.Close()
	if err := p.Publish(context.Background(), events.Event{ID: "1", Type: "x"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
}

func TestDispatcherPublishesAndStopsOnContext(t *testing.T) {
	o := events.NewOutbox()
	p := events.NewLoggingPublisher(newTestLogger())
	d := events.NewDispatcher(o, p, newTestLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		d.Run(ctx)
		close(done)
	}()

	if err := o.Enqueue(ctx, events.Event{Type: "user.created", Topic: "users"}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if o.Len() == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	_ = o.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("dispatcher did not stop")
	}
	if o.Len() != 0 {
		t.Fatalf("expected empty outbox, got %d", o.Len())
	}
}
