package utils

import "context"

// DevMailer is the fallback used when SMTP is not configured. It logs the full
// email (including the deep link and its token) at INFO instead of sending it,
// so registration, verification and password reset can be exercised end to end
// with zero credentials. It never returns an error, which is what makes it a
// safe automatic fallback rather than a startup failure.
type DevMailer struct{}

// NewDevMailer returns the log-only mailer.
func NewDevMailer() *DevMailer { return &DevMailer{} }

// Mode implements Mailer.
func (m *DevMailer) Mode() string { return "dev (log-only)" }

// Send implements Mailer by logging the message. The body is logged verbatim so
// the verification/reset link is copy-pasteable from the server logs.
func (m *DevMailer) Send(_ context.Context, msg Message) error {
	Log.Infof("[MAILER:dev] email to=%q subject=%q\n%s", msg.To, msg.Subject, msg.Text)
	return nil
}
