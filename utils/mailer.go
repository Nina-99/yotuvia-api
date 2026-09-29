package utils

// Transactional email abstraction.
//
// The concrete provider is deliberately NOT baked in anywhere above this file:
// every major email provider (Resend, Amazon SES, SendGrid, Postmark, Gmail,
// Mailgun, ...) speaks SMTP, so choosing one later is a matter of credentials in
// the environment — no code change. The service layer depends only on the
// Mailer interface; tests inject a recording fake; an unconfigured environment
// falls back to a log-only dev mailer so the flows stay testable end to end
// without credentials.
//
// The SMTP transport lives in mailer_smtp.go and is the only file that imports
// a mail library. Replacing it with a provider SDK later means writing one new
// Mailer implementation and pointing the factory at it.

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Message is a provider-agnostic outbound email. HTML may be empty, in which
// case only the plain-text alternative is sent.
type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

// Mailer sends transactional email. Implementations must be safe for concurrent
// use.
type Mailer interface {
	// Send delivers msg. A nil error means the provider accepted the message.
	Send(ctx context.Context, msg Message) error
	// Mode is a short human label for the active transport, logged once at
	// startup ("smtp", "dev (log-only)", "fake").
	Mode() string
}

// Environment variables for the SMTP transport. Documented here because there
// is no checked-in .env.example by default:
//
//	SMTP_HOST      SMTP server hostname. Empty/unset selects the log-only dev
//	               mailer (the automatic fallback).
//	SMTP_PORT      SMTP port. Default 587 (submission/STARTTLS).
//	SMTP_USER      SMTP username. Optional for relays without auth.
//	SMTP_PASSWORD  SMTP password.
//	SMTP_FROM      From address. Defaults to SMTP_USER.
//	SMTP_TLS_MODE  "starttls" (default), "tls" (implicit TLS) or "none".
const (
	envSMTPHost    = "SMTP_HOST"
	envSMTPPort    = "SMTP_PORT"
	envSMTPUser    = "SMTP_USER"
	envSMTPPass    = "SMTP_PASSWORD"
	envSMTPFrom    = "SMTP_FROM"
	envSMTPTLSMode = "SMTP_TLS_MODE"
)

// defaultMailer backs GetMailer/SetDefaultMailer. The process wires it once in
// main; tests may replace it, though passing a Mailer into the service is the
// preferred injection point.
var (
	defaultMailer   Mailer
	defaultMailerMu sync.Mutex
)

// SetDefaultMailer installs the process-wide mailer.
func SetDefaultMailer(m Mailer) {
	defaultMailerMu.Lock()
	defer defaultMailerMu.Unlock()
	defaultMailer = m
}

// GetMailer returns the process-wide mailer, lazily building the env-configured
// one on first use (dev fallback when SMTP is unconfigured). It never returns
// nil and never panics.
func GetMailer() Mailer {
	defaultMailerMu.Lock()
	defer defaultMailerMu.Unlock()
	if defaultMailer == nil {
		m, err := MailerFromEnv()
		if err != nil {
			Log.Errorf("[MAILER] invalid SMTP configuration, falling back to dev (log-only): %v", err)
			defaultMailer = NewDevMailer()
		} else {
			defaultMailer = m
		}
	}
	return defaultMailer
}

// MailerFromEnv selects the transport from the environment. With SMTP_HOST
// unset it returns the log-only dev mailer (not an error). A *configured* but
// malformed SMTP setup returns an error so a typo fails loudly instead of
// silently dropping mail on the floor.
func MailerFromEnv() (Mailer, error) {
	host := strings.TrimSpace(os.Getenv(envSMTPHost))
	if host == "" {
		return NewDevMailer(), nil
	}

	port := 587
	if raw := strings.TrimSpace(os.Getenv(envSMTPPort)); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 || n > 65535 {
			return nil, fmt.Errorf("invalid SMTP_PORT %q", raw)
		}
		port = n
	}

	from := strings.TrimSpace(os.Getenv(envSMTPFrom))
	if from == "" {
		from = strings.TrimSpace(os.Getenv(envSMTPUser))
	}
	if from == "" {
		return nil, fmt.Errorf("SMTP_FROM (or SMTP_USER) must be set when SMTP_HOST is configured")
	}

	tlsMode := strings.ToLower(strings.TrimSpace(os.Getenv(envSMTPTLSMode)))
	if tlsMode == "" {
		tlsMode = "starttls"
	}
	switch tlsMode {
	case "starttls", "tls", "none":
	default:
		return nil, fmt.Errorf("invalid SMTP_TLS_MODE %q (want starttls, tls or none)", tlsMode)
	}

	return NewSMTPMailer(SMTPConfig{
		Host:     host,
		Port:     port,
		Username: strings.TrimSpace(os.Getenv(envSMTPUser)),
		Password: os.Getenv(envSMTPPass),
		From:     from,
		TLSMode:  tlsMode,
	})
}
