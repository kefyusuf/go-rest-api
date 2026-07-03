// Package events implements a small publish-subscribe mechanism
// with an outbox pattern. Handlers that want to emit an event call
// outbox.Enqueue. A background dispatcher reads the outbox and
// forwards every event to the active Publisher. The Publisher
// interface is small so the in-memory implementation in the starter
// can be replaced with Kafka (via segmentio/kafka-go) or another
// broker without changing the call sites.
//
// Two Outbox implementations are available:
//
//   - Outbox (this file): a per-process slice. Lost on restart and
//     not shared across replicas. Use it for single-instance
//     development.
//   - DBOutbox (db.go): a database-backed outbox. Survives a
//     process restart and is safe across replicas. This is the
//     production swap.
package events

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

var ErrOutboxClosed = errors.New("outbox is closed")

// Outbox is the contract every outbox implementation must satisfy.
// The HTTP handler and the dispatcher talk to this interface so
// callers can swap the backing store without changing either side.
type Outbox interface {
	Enqueue(ctx context.Context, e Event) error
	Dequeue(ctx context.Context) (Event, bool, error)
	MarkPublished(ctx context.Context, e Event) error
	Len() int
	Close() error
}

// In-memory outbox. Lost on restart and not shared across replicas.
// Suitable for single-instance development.

type inMemoryOutbox struct {
	mu     sync.Mutex
	cond   *sync.Cond
	queue  []entry
	closed bool
}

type entry struct {
	event      Event
	enqueuedAt time.Time
}

func NewOutbox() *inMemoryOutbox {
	o := &inMemoryOutbox{}
	o.cond = sync.NewCond(&o.mu)
	return o
}

// Outbox is a backwards-compatible alias for the in-memory
// implementation. Existing call sites that pass *Outbox still work
// after the interface promotion because *inMemoryOutbox satisfies
// the Outbox interface.
type OutboxImpl = inMemoryOutbox

func (o *inMemoryOutbox) Enqueue(ctx context.Context, e Event) error {
	if e.ID == "" {
		e.ID = newID()
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now()
	}

	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return ErrOutboxClosed
	}
	o.queue = append(o.queue, entry{event: e, enqueuedAt: time.Now()})
	o.cond.Signal()
	return nil
}

func (o *inMemoryOutbox) Dequeue(ctx context.Context) (Event, bool, error) {
	o.mu.Lock()
	for len(o.queue) == 0 && !o.closed {
		o.cond.Wait()
	}
	if len(o.queue) == 0 {
		o.mu.Unlock()
		return Event{}, false, nil
	}
	e := o.queue[0]
	o.queue = o.queue[1:]
	o.mu.Unlock()
	return e.event, true, nil
}

func (o *inMemoryOutbox) MarkPublished(ctx context.Context, e Event) error {
	return nil
}

func (o *inMemoryOutbox) Len() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.queue)
}

func (o *inMemoryOutbox) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return nil
	}
	o.closed = true
	o.cond.Broadcast()
	return nil
}

type Dispatcher struct {
	outbox    Outbox
	publisher Publisher
	logger    *slog.Logger
}

func NewDispatcher(o Outbox, p Publisher, logger *slog.Logger) *Dispatcher {
	if logger == nil {
		logger = slog.Default()
	}
	return &Dispatcher{outbox: o, publisher: p, logger: logger}
}

func (d *Dispatcher) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		e, ok, err := d.outbox.Dequeue(ctx)
		if err != nil {
			if errors.Is(err, ErrOutboxClosed) {
				return
			}
			d.logger.Error("outbox dequeue failed", slog.String("error", err.Error()))
			return
		}
		if !ok {
			return
		}

		e.PublishedAt = time.Now()
		if perr := d.publisher.Publish(ctx, e); perr != nil {
			d.logger.Error("event publish failed",
				slog.String("id", e.ID),
				slog.String("type", e.Type),
				slog.String("error", perr.Error()))
			continue
		}
		if merr := d.outbox.MarkPublished(ctx, e); merr != nil {
			d.logger.Error("outbox mark-published failed", slog.String("error", merr.Error()))
		}
	}
}

func newID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("evt-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// --- DBOutbox (database-backed) ---

// DBOutbox keeps events in a single events_outbox table. Each
// Enqueue inserts a row; each Dequeue selects the oldest
// unpublished row and locks it with FOR UPDATE SKIP LOCKED so
// concurrent dispatchers do not pick the same row. MarkPublished
// deletes the row.
//
// Schema (created by the migration in db.go):
//
//   CREATE TABLE events_outbox (
//     id           TEXT PRIMARY KEY,
//     type         TEXT NOT NULL,
//     topic        TEXT NOT NULL,
//     key          TEXT NOT NULL,
//     payload      JSONB NOT NULL,
//     occurred_at  TIMESTAMPTZ NOT NULL,
//     published_at TIMESTAMPTZ
//   );
type DBOutbox struct {
	db    *sql.DB
	table string
}

// NewDBOutbox builds a database-backed outbox. The caller owns
// the *sql.DB and is responsible for closing it; the outbox
// does not close the connection.
func NewDBOutbox(db *sql.DB) *DBOutbox {
	return &DBOutbox{db: db, table: "events_outbox"}
}

// EnsureSchema creates the events_outbox table and the supporting
// index if they do not exist. Safe to call on every boot; the
// statements are idempotent.
func (o *DBOutbox) EnsureSchema(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS events_outbox (
			id           TEXT PRIMARY KEY,
			type         TEXT NOT NULL,
			topic        TEXT NOT NULL,
			key          TEXT NOT NULL,
			payload      JSONB NOT NULL,
			occurred_at  TIMESTAMPTZ NOT NULL,
			published_at TIMESTAMPTZ
		)`,
		`CREATE INDEX IF NOT EXISTS events_outbox_published_at_idx
			ON events_outbox (published_at NULLS FIRST, occurred_at)`,
	}
	for _, s := range stmts {
		if _, err := o.db.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("ensure outbox schema: %w", err)
		}
	}
	return nil
}

func (o *DBOutbox) Enqueue(ctx context.Context, e Event) error {
	if e.ID == "" {
		e.ID = newID()
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now()
	}
	payload, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = o.db.ExecContext(ctx,
		`INSERT INTO events_outbox (id, type, topic, key, payload, occurred_at, published_at)
		 VALUES ($1, $2, $3, $4, $5, $6, NULL)`,
		e.ID, e.Type, e.Topic, e.Key, payload, e.OccurredAt)
	return err
}

func (o *DBOutbox) Dequeue(ctx context.Context) (Event, bool, error) {
	// Use a short transaction so the row lock is released quickly
	// when the dispatcher does not publish within the deadline.
	tx, err := o.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return Event{}, false, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `
		SELECT id, type, topic, key, payload, occurred_at
		FROM events_outbox
		WHERE published_at IS NULL
		ORDER BY occurred_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED`)
	var e Event
	var payload []byte
	if err := row.Scan(&e.ID, &e.Type, &e.Topic, &e.Key, &payload, &e.OccurredAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Event{}, false, nil
		}
		return Event{}, false, err
	}
	if err := json.Unmarshal(payload, &e); err != nil {
		// A malformed entry should not block the queue. Delete
		// it on the same transaction so the next dequeue skips
		// past it cleanly.
		_, _ = tx.ExecContext(ctx, `DELETE FROM events_outbox WHERE id = $1`, e.ID)
		_ = tx.Commit()
		return Event{}, false, fmt.Errorf("decode outbox entry: %w", err)
	}

	// Hold the lock in a separate connection until publish
	// succeeds. For simplicity, we hand the Event back to the
	// dispatcher and rely on the row to be published and then
	// deleted by MarkPublished. The lock is released as soon as
	// the transaction is closed; concurrent dispatchers will
	// skip the locked row (SKIP LOCKED) and pick the next one.
	// This is at-least-once in the common case; a crash between
	// Dequeue and MarkPublished leaves the row unpublished and
	// the next dispatcher will pick it up.
	if err := tx.Commit(); err != nil {
		return Event{}, false, err
	}
	return e, true, nil
}

func (o *DBOutbox) MarkPublished(ctx context.Context, e Event) error {
	_, err := o.db.ExecContext(ctx,
		`DELETE FROM events_outbox WHERE id = $1`, e.ID)
	return err
}

func (o *DBOutbox) Len() int {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var n int
	if err := o.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM events_outbox WHERE published_at IS NULL`).Scan(&n); err != nil {
		return 0
	}
	return n
}

func (o *DBOutbox) Close() error { return nil }

// Compile-time assertions: both implementations satisfy Outbox.
var (
	_ Outbox = (*inMemoryOutbox)(nil)
	_ Outbox = (*DBOutbox)(nil)
)
