// Package events implements a small publish-subscribe mechanism
// with an outbox-style pattern. Handlers that want to emit an
// event call outbox.Enqueue. A background dispatcher reads the
// outbox and forwards every event to the active Publisher. The
// Publisher interface is small so the in-memory implementation in
// the starter can be replaced with Kafka or another broker
// without changing the call sites.
package events

import (
	"context"
	"time"
)

type Event struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Topic       string    `json:"topic"`
	Key         string    `json:"key,omitempty"`
	Payload     []byte    `json:"payload"`
	OccurredAt  time.Time `json:"occurredAt"`
	PublishedAt time.Time `json:"publishedAt,omitempty"`
}

type Publisher interface {
	Publish(ctx context.Context, event Event) error
	Close() error
}

type Subscriber interface {
	Subscribe(topic string, handler func(ctx context.Context, event Event) error) error
}
