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

func kafkaTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestKafkaPublisherRejectsEmptyBrokers(t *testing.T) {
	_, err := events.NewKafkaPublisher(events.KafkaConfig{}, kafkaTestLogger())
	if !errors.Is(err, events.ErrNoBrokers) {
		t.Fatalf("expected ErrNoBrokers, got %v", err)
	}
}

func TestKafkaPublisherFailsOnUnreachableBroker(t *testing.T) {
	p, err := events.NewKafkaPublisher(events.KafkaConfig{
		Brokers:      []string{"127.0.0.1:1"},
		WriteTimeout: 200 * time.Millisecond,
	}, kafkaTestLogger())
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	defer p.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := p.Publish(ctx, events.Event{
		ID:    "e1",
		Type:  "test.event",
		Topic: "go-rest-api.events",
		Key:   "k1",
	}); err == nil {
		t.Fatal("expected publish to fail with unreachable broker")
	}
}
