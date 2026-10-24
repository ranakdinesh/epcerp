package security

import (
	"context"
	"strings"
	"testing"
)

type captureMail struct{ recipient, subject, message string }

func (*captureMail) Ready() bool { return true }
func (m *captureMail) Send(_ context.Context, to, subject, message string) error {
	m.recipient = to
	m.subject = subject
	m.message = message
	return nil
}
func TestRecoveryContentUsesSharedMail(t *testing.T) {
	m := &captureMail{}
	r := NewRecoveryMail(m, "https://erp.vishaltechnopower.in")
	if e := r.SendRecovery(context.Background(), "person@example.invalid", "fixture-token"); e != nil {
		t.Fatal(e)
	}
	if m.recipient != "person@example.invalid" || !strings.Contains(m.message, "https://erp.vishaltechnopower.in/reset-password#token=fixture-token") || strings.Contains(m.message, "?token=") {
		t.Fatal("shared mail recovery contract changed")
	}
}
