package security

import (
	"context"
	"net/url"
)

type MailSender interface {
	Ready() bool
	Send(context.Context, string, string, string) error
}

// RecoveryMail supplies Auth content; all transport and formatting are shared infrastructure.
type RecoveryMail struct {
	sender   MailSender
	resetURL string
}

func NewRecoveryMail(sender MailSender, publicURL string) *RecoveryMail {
	return &RecoveryMail{sender, publicURL + "/reset-password"}
}
func (m *RecoveryMail) Ready() bool { return m.sender.Ready() }
func (m *RecoveryMail) SendRecovery(ctx context.Context, email, token string) error {
	return m.sender.Send(ctx, email, "Reset your epcERP password", "A password reset was requested for your epcERP account. Open the secure link below to choose a new password.\n\n"+m.resetURL+"#token="+url.QueryEscape(token)+"\n\nThis link expires in 15 minutes and can be used once. If you did not request this, ignore this email. Your password has not changed.")
}
