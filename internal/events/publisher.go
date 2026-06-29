package events

import (
	"context"
	"log/slog"
	"sync"
)

type LoggingPublisher struct {
	logger *slog.Logger
	mu     sync.Mutex
}

func NewLoggingPublisher(logger *slog.Logger) *LoggingPublisher {
	if logger == nil {
		logger = slog.Default()
	}
	return &LoggingPublisher{logger: logger}
}

func (p *LoggingPublisher) Publish(_ context.Context, event Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.logger.Info("event published",
		slog.String("id", event.ID),
		slog.String("type", event.Type),
		slog.String("topic", event.Topic),
		slog.String("key", event.Key))
	return nil
}

func (p *LoggingPublisher) Close() error { return nil }
