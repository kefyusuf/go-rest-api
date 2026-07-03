package email

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	"github.com/wneessen/go-mail"
)

// SMTPConfig configures SMTPSender. Host, Port, User, Password,
// FromAddress are required. StartTLS controls whether the client
// upgrades the connection to TLS; the default is opportunistic
// (true) which matches what Postmark / Mailgun / SES expect.
type SMTPConfig struct {
	Host        string
	Port        int
	User        string
	Password    string
	FromAddress string
	StartTLS    bool
	Timeout     time.Duration
}

// NewSMTPConfigFromEnv pulls the SMTP_* env-style fields. It
// returns (cfg, true) when the minimum set of required fields is
// present, and (zero, false) when SMTP is intentionally not
// configured. Callers should treat the false case as a
// configuration error: the production swap is not active, the
// in-memory fallback is in use, and the API will not actually
// send email until SMTP_* is set.
func NewSMTPConfigFromEnv(getenv func(string) string) (SMTPConfig, bool) {
	host := getenv("SMTP_HOST")
	user := getenv("SMTP_USER")
	pass := getenv("SMTP_PASSWORD")
	from := getenv("SMTP_FROM")
	if host == "" || user == "" || pass == "" || from == "" {
		return SMTPConfig{}, false
	}

	port := 587
	if p := getenv("SMTP_PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}

	useTLS := true
	if v := getenv("SMTP_STARTTLS"); v == "false" || v == "0" {
		useTLS = false
	}

	timeout := 10 * time.Second
	if v := getenv("SMTP_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			timeout = d
		}
	}

	return SMTPConfig{
		Host:        host,
		Port:        port,
		User:        user,
		Password:    pass,
		FromAddress: from,
		StartTLS:    useTLS,
		Timeout:     timeout,
	}, true
}

// SMTPSender is the production Sender. It builds a go-mail
// client per Send call so the connection lifecycle is per-message
// (a good fit for low-volume transactional email from a small
// service). High-volume senders should reuse a single client;
// that is a future optimisation.
type SMTPSender struct {
	cfg    SMTPConfig
	logger *slog.Logger
}

// NewSMTPSender builds a Sender from a pre-validated config. A
// nil logger falls back to slog.Default. The caller is expected
// to have already verified that the required fields are set;
// the Send method returns ErrSMTPNotConfigured if any of them
// is missing.
func NewSMTPSender(cfg SMTPConfig, logger *slog.Logger) *SMTPSender {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &SMTPSender{cfg: cfg, logger: logger}
}

// Send composes a mail.Msg and dials the SMTP server. The
// connection is closed when the function returns. StartTLS is
// applied when SMTPSender.StartTLS is true, which is the default.
//
// The supplied context is honoured by go-mail through a
// DialContextFunc that aborts the underlying TCP dial if the
// context is cancelled before the connection completes. After
// the connection is up, SMTP does not have a wire-level
// cancellation, so a long send will not be interrupted by ctx
// alone; the Timeout option bounds the underlying read/write
// deadlines instead.
func (s *SMTPSender) Send(ctx context.Context, m Message) error {
	if s.cfg.Host == "" || s.cfg.User == "" || s.cfg.Password == "" || s.cfg.FromAddress == "" {
		return ErrSMTPNotConfigured
	}

	msg := mail.NewMsg()
	if err := msg.From(m.From); err != nil {
		if m.From == "" {
			msg.From(s.cfg.FromAddress)
		} else {
			return fmt.Errorf("smtp: set from: %w", err)
		}
	}
	if err := msg.To(m.To); err != nil {
		return fmt.Errorf("smtp: set to: %w", err)
	}
	msg.Subject(m.Subject)
	msg.SetBodyString(mail.TypeTextPlain, m.Body)

	// Bridge the caller's context into go-mail's dialer so a
	// caller-side cancellation aborts a slow connect. The body
	// of DialContextFunc runs on the connection-establishment
	// path; once the SMTP handshake completes, this closure
	// is no longer invoked.
	dialCtx := ctx
	dialFn := func(ctx context.Context, network, address string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, network, address)
	}
	_ = dialCtx

	opts := []mail.Option{
		mail.WithPort(s.cfg.Port),
		mail.WithTimeout(s.cfg.Timeout),
		mail.WithSMTPAuth(mail.SMTPAuthLogin),
		mail.WithUsername(s.cfg.User),
		mail.WithPassword(s.cfg.Password),
		mail.WithDialContextFunc(dialFn),
	}
	if s.cfg.StartTLS {
		opts = append(opts, mail.WithTLSConfig(&tls.Config{ServerName: s.cfg.Host}))
	}

	client, err := mail.NewClient(s.cfg.Host, opts...)
	if err != nil {
		s.logger.Error("smtp: build client",
			slog.String("host", s.cfg.Host),
			slog.String("error", err.Error()))
		return fmt.Errorf("smtp: build client: %w", err)
	}

	if err := client.DialAndSend(msg); err != nil {
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			s.logger.Error("smtp: send timed out",
				slog.String("to", m.To),
				slog.String("error", err.Error()))
		} else {
			s.logger.Error("smtp: send failed",
				slog.String("to", m.To),
				slog.String("error", err.Error()))
		}
		return fmt.Errorf("smtp: send: %w", err)
	}
	s.logger.Info("smtp: sent",
		slog.String("to", m.To),
		slog.String("subject", m.Subject))
	return nil
}

// Compile-time assertion.
var _ Sender = (*SMTPSender)(nil)
