package events

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

var ErrOutboxClosed = errors.New("outbox is closed")

type entry struct {
	event      Event
	enqueuedAt time.Time
}

type Outbox struct {
	mu     sync.Mutex
	cond   *sync.Cond
	queue  []entry
	closed bool
}

func NewOutbox() *Outbox {
	o := &Outbox{}
	o.cond = sync.NewCond(&o.mu)
	return o
}

func (o *Outbox) Enqueue(ctx context.Context, e Event) error {
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

func (o *Outbox) Dequeue(ctx context.Context) (Event, bool, error) {
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

func (o *Outbox) MarkPublished(ctx context.Context, e Event) error {
	return nil
}

func (o *Outbox) Len() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.queue)
}

func (o *Outbox) Close() error {
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
	outbox    *Outbox
	publisher Publisher
	logger    *slog.Logger
}

func NewDispatcher(o *Outbox, p Publisher, logger *slog.Logger) *Dispatcher {
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
