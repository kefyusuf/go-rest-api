package email_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"go-lang/internal/email"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestLogSenderDoesNotError(t *testing.T) {
	s := email.NewLogSender(discardLogger())
	if err := s.Send(context.Background(), email.Message{
		From:    "noreply@example.com",
		To:      "user@example.com",
		Subject: "Welcome",
		Body:    "Hello",
	}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestLogSenderImplementsSender(t *testing.T) {
	// Compile-time check via interface assignment. The build
	// itself is the assertion.
	var _ email.Sender = (*email.LogSender)(nil)
}

func TestSMTPSenderRejectsMissingConfig(t *testing.T) {
	s := email.NewSMTPSender(email.SMTPConfig{}, discardLogger())
	err := s.Send(context.Background(), email.Message{To: "user@example.com"})
	if err != email.ErrSMTPNotConfigured {
		t.Fatalf("expected ErrSMTPNotConfigured, got %v", err)
	}
}
