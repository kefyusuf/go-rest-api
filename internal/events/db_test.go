package events_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"go-lang/internal/events"
)

// postgresDSN returns a Postgres connection string. The test
// is skipped when the DATABASE_URL env var is not set, so the
// suite stays green on machines without a running Postgres.
func postgresDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping DBOutbox test")
	}
	return dsn
}

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", postgresDSN(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Skipf("postgres not reachable, skipping: %v", err)
	}
	return db
}

func TestDBOutboxEnqueueDequeueMarkPublished(t *testing.T) {
	db := openDB(t)
	defer db.Close()

	box := events.NewDBOutbox(db)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := box.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	id := "test-" + time.Now().Format("150405.000000000")
	// Clean up any leftover row from a previous run.
	defer db.ExecContext(ctx, `DELETE FROM events_outbox WHERE id = $1`, id)

	event := events.Event{
		ID:         id,
		Type:       "user.created",
		Topic:      "users",
		Key:        "1",
		Payload:    []byte(`{"id":1}`),
		OccurredAt: time.Now(),
	}

	if err := box.Enqueue(ctx, event); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	got, ok, err := box.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if !ok {
		t.Fatal("expected a queued event")
	}
	if got.ID != id {
		t.Fatalf("expected id %q, got %q", id, got.ID)
	}
	if got.Type != "user.created" {
		t.Fatalf("expected type user.created, got %q", got.Type)
	}
	if string(got.Payload) != `{"id":1}` {
		t.Fatalf("payload mismatch: %q", string(got.Payload))
	}

	if err := box.MarkPublished(ctx, got); err != nil {
		t.Fatalf("mark published: %v", err)
	}

	// After mark-published the row is gone, so a fresh dequeue
	// must be empty.
	got, ok, err = box.Dequeue(ctx)
	if err != nil {
		t.Fatalf("second dequeue: %v", err)
	}
	if ok {
		t.Fatalf("expected queue empty after mark-published, got %+v", got)
	}
}

func TestDBOutboxLenCountsUnpublished(t *testing.T) {
	db := openDB(t)
	defer db.Close()

	box := events.NewDBOutbox(db)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := box.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	prefix := "test-len-" + time.Now().Format("150405.000000000")
	defer func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM events_outbox WHERE id LIKE $1`, prefix+"%")
	}()

	if err := box.Enqueue(ctx, events.Event{ID: prefix + "-1", Type: "t", OccurredAt: time.Now()}); err != nil {
		t.Fatalf("enqueue 1: %v", err)
	}
	if err := box.Enqueue(ctx, events.Event{ID: prefix + "-2", Type: "t", OccurredAt: time.Now()}); err != nil {
		t.Fatalf("enqueue 2: %v", err)
	}

	if l := box.Len(); l != 2 {
		t.Fatalf("expected len 2, got %d", l)
	}
}
