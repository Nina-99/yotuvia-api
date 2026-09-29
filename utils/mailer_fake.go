package utils

import (
	"context"
	"sync"
)

// FakeMailer records sent messages in memory so tests can assert on the link
// and token without a real provider. Set SendErr to simulate a provider
// failure (which must never fail registration).
type FakeMailer struct {
	mu        sync.Mutex
	sent      []Message
	SendErr   error
	callCount int
}

// NewFakeMailer returns an empty recording mailer.
func NewFakeMailer() *FakeMailer { return &FakeMailer{} }

// Mode implements Mailer.
func (m *FakeMailer) Mode() string { return "fake" }

// Send implements Mailer. On SendErr the error is returned and the message is
// not recorded, mirroring a real transport failure.
func (m *FakeMailer) Send(_ context.Context, msg Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	if m.SendErr != nil {
		return m.SendErr
	}
	m.sent = append(m.sent, msg)
	return nil
}

// Messages returns a copy of everything sent so far.
func (m *FakeMailer) Messages() []Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Message, len(m.sent))
	copy(out, m.sent)
	return out
}

// Last returns the most recent message, or false when nothing was sent.
func (m *FakeMailer) Last() (Message, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sent) == 0 {
		return Message{}, false
	}
	return m.sent[len(m.sent)-1], true
}

// CallCount returns how many times Send was invoked, including failures.
func (m *FakeMailer) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}
