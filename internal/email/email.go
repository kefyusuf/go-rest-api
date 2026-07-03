// Package email is the outbound-mail abstraction for the
// application. Handlers depend on the Sender interface so the
// concrete implementation can be swapped without touching them.
//
// Two implementations are available:
//
//   - LogSender: writes the message to a slog logger. The default
//     for single-instance development. The message never leaves
//     the process.
//   - SMTPSender: relays the message through a real SMTP server
//     (Postmark, Mailgun, AWS SES, your own Postfix, etc.) using
//     the wneessen/go-mail client. The production swap; configure
//     with the SMTP_* env vars listed in docs/OPERATIONS.md.
package email

import (
	"context"
	"errors"
	"log/slog"
)

// Message is the minimum data a Sender needs.
type Message struct {
	From    string
	To      string
	Subject string
	Body    string
}

// Sender is the contract every outbound-mail implementation
// must satisfy. Implementations should be safe for concurrent use.
type Sender interface {
	Send(ctx context.Context, m Message) error
}

// LogSender prints the message to the supplied logger. Useful in
// development and in tests; not appropriate for production
// because nothing actually reaches the recipient.
type LogSender struct {
	logger *slog.Logger
}

// NewLogSender returns a Sender that logs every message at INFO
// level. A nil logger falls back to slog.Default.
func NewLogSender(logger *slog.Logger) *LogSender {
	if logger == nil {
		logger = slog.Default()
	}
	return &LogSender{logger: logger}
}

// Send logs the message and returns nil. The error path exists
// only so LogSender satisfies the Sender interface.
func (l *LogSender) Send(_ context.Context, m Message) error {
	l.logger.Info("email (log-only)",
		slog.String("from", m.From),
		slog.String("to", m.To),
		slog.String("subject", m.Subject))
	return nil
}

// Compile-time assertion.
var _ Sender = (*LogSender)(nil)

// ErrSMTPNotConfigured is returned by SMTPSender when the
// required SMTP_* env vars are missing. Callers should treat
// this as a configuration error, not a transient failure.
var ErrSMTPNotConfigured = errors.New("smtp: not configured")
