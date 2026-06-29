package events

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
)

// KafkaPublisher implements Publisher by writing events to a Kafka
// broker via segmentio/kafka-go. It is the production swap for
// LoggingPublisher: both implement the same interface, so main.go
// picks one at startup based on whether KAFKA_BROKERS is set.
type KafkaPublisher struct {
	writer *kafka.Writer
	logger *slog.Logger
	topic  string
}

// KafkaConfig configures the KafkaPublisher. Brokers must be set;
// the other fields fall back to safe defaults.
type KafkaConfig struct {
	Brokers      []string
	Topic        string
	WriteTimeout time.Duration
	BatchTimeout time.Duration
	RequiredAcks kafka.RequiredAcks
}

func NewKafkaPublisher(cfg KafkaConfig, logger *slog.Logger) (*KafkaPublisher, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if len(cfg.Brokers) == 0 {
		return nil, ErrNoBrokers
	}
	if cfg.Topic == "" {
		cfg.Topic = "go-rest-api.events"
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 10 * time.Second
	}
	if cfg.BatchTimeout == 0 {
		cfg.BatchTimeout = 50 * time.Millisecond
	}
	if cfg.RequiredAcks == 0 {
		cfg.RequiredAcks = kafka.RequireAll
	}
	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        cfg.Topic,
		Balancer:     &kafka.Hash{}, // partition by event.Key when set
		RequiredAcks: cfg.RequiredAcks,
		WriteTimeout: cfg.WriteTimeout,
		BatchTimeout: cfg.BatchTimeout,
		Async:        false, // synchronous Publish so the caller can react to errors
		Logger:       kafka.LoggerFunc(func(msg string, args ...any) { logger.Debug("kafka", "msg", msg, "args", args) }),
		ErrorLogger:  kafka.LoggerFunc(func(msg string, args ...any) { logger.Warn("kafka", "msg", msg, "args", args) }),
	}
	return &KafkaPublisher{writer: writer, logger: logger, topic: cfg.Topic}, nil
}

func (p *KafkaPublisher) Publish(ctx context.Context, event Event) error {
	topic := p.topic
	if event.Topic != "" {
		topic = event.Topic
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg := kafka.Message{
		Topic: topic,
		Key:   []byte(event.Key),
		Value: data,
		Time:  event.OccurredAt,
		Headers: []kafka.Header{
			{Key: "event-id", Value: []byte(event.ID)},
			{Key: "event-type", Value: []byte(event.Type)},
		},
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		p.logger.Error("kafka publish failed",
			slog.String("topic", topic),
			slog.String("key", event.Key),
			slog.String("error", err.Error()))
		return err
	}
	return nil
}

func (p *KafkaPublisher) Close() error {
	if p.writer == nil {
		return nil
	}
	return p.writer.Close()
}

var ErrNoBrokers = errors.New("at least one broker is required")
