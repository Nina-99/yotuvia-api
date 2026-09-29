package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/wneessen/go-mail"
)

// SMTPConfig is the credentials/transport description for SMTPMailer.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	// TLSMode is "starttls" (default, submission port), "tls" (implicit TLS) or
	// "none". It is validated by MailerFromEnv before construction.
	TLSMode string
}

// SMTPMailer sends mail through an SMTP relay using go-mail.
//
// Why go-mail (github.com/wneessen/go-mail) over stdlib net/smtp: net/smtp is
// frozen ("no new features") and only implements plain/CRAM-MD5 auth plus
// STARTTLS, with no convenient multipart helper or implicit-TLS (SMTPS) mode —
// exactly the cases providers differ on. go-mail is actively maintained, pure
// Go, uses no provider SDK, builds HTML+text multipart for us, and supports
// every TLS/auth mode we need. One dependency replaces any future provider SDK.
type SMTPMailer struct {
	cfg    SMTPConfig
	client *mail.Client
}

// NewSMTPMailer builds a Mailer for cfg. It validates the configuration and
// constructs the client eagerly so a bad TLS mode or address fails at startup,
// not on the first user's registration.
func NewSMTPMailer(cfg SMTPConfig) (*SMTPMailer, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("smtp: host is required")
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return nil, fmt.Errorf("smtp: invalid port %d", cfg.Port)
	}
	if cfg.From == "" {
		return nil, fmt.Errorf("smtp: from address is required")
	}

	opts := []mail.Option{
		mail.WithPort(cfg.Port),
		mail.WithTimeout(15 * time.Second),
	}

	switch cfg.TLSMode {
	case "", "starttls":
		opts = append(opts, mail.WithTLSPolicy(mail.TLSMandatory))
	case "tls":
		// Implicit TLS (SMTPS): encrypt from the first byte, typically port 465.
		opts = append(opts, mail.WithSSL())
	case "none":
		opts = append(opts, mail.WithTLSPolicy(mail.NoTLS))
	default:
		return nil, fmt.Errorf("smtp: invalid TLS mode %q", cfg.TLSMode)
	}

	if cfg.Username != "" {
		opts = append(opts,
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithUsername(cfg.Username),
			mail.WithPassword(cfg.Password),
		)
	}

	client, err := mail.NewClient(cfg.Host, opts...)
	if err != nil {
		return nil, fmt.Errorf("smtp: failed to build client: %w", err)
	}
	return &SMTPMailer{cfg: cfg, client: client}, nil
}

// Mode implements Mailer.
func (m *SMTPMailer) Mode() string { return "smtp" }

// Send implements Mailer, building a multipart/alternative message with a
// plain-text body and, when provided, an HTML alternative.
func (m *SMTPMailer) Send(ctx context.Context, msg Message) error {
	if msg.To == "" || msg.Subject == "" {
		return fmt.Errorf("smtp: message needs a recipient and a subject")
	}

	m2 := mail.NewMsg()
	if err := m2.From(m.cfg.From); err != nil {
		return fmt.Errorf("smtp: invalid from address: %w", err)
	}
	if err := m2.To(msg.To); err != nil {
		return fmt.Errorf("smtp: invalid recipient %q: %w", msg.To, err)
	}
	m2.Subject(msg.Subject)
	m2.SetBodyString(mail.TypeTextPlain, msg.Text)
	if msg.HTML != "" {
		m2.AddAlternativeString(mail.TypeTextHTML, msg.HTML)
	}

	if err := m.client.DialAndSendWithContext(ctx, m2); err != nil {
		return fmt.Errorf("smtp: send failed: %w", err)
	}
	return nil
}
