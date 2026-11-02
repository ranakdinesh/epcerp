package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/ports"
)

func (s *AccountService) WithEmailSender(mail ports.EmailSender, publicURL string) *AccountService {
	s.mail = mail
	s.publicURL = publicURL
	return s
}
func (s *UserService) WithEmailSender(mail ports.EmailSender, publicURL string) *UserService {
	s.mail = mail
	s.publicURL = publicURL
	return s
}
func stageEmail(ctx context.Context, r ports.EmailChangeRepository, t ports.Tokens, u domain.User, email, actor string) (string, error) {
	if email == u.Email {
		return "", &domain.Error{Kind: domain.Invalid, Detail: "Enter a different email address."}
	}
	if e := r.CancelEmailChanges(ctx, u.ID); e != nil {
		return "", e
	}
	if e := r.CancelCredentialForUser(ctx, u.ID); e != nil {
		return "", e
	}
	raw, hash := t.Refresh()
	e := r.SaveEmailChange(ctx, domain.EmailChange{Hash: hash, UserID: u.ID, Email: email, PreviousEmail: u.Email, CredentialVersion: u.CredentialVersion, ActorID: actor, ExpiresAt: domain.At(t.Now() + 1800)})
	return raw, e
}
func sendVerification(ctx context.Context, m ports.EmailSender, publicURL, email, raw string) error {
	if e := m.Send(ctx, email, "Verify your new epcERP email address", "An email address change was requested for your epcERP account. Confirm this address using the link below. Your current sign-in email remains unchanged until you confirm.\n\n"+publicURL+"/verify-email#token="+raw+"\n\nThis link expires in 30 minutes and can be used once. Ignore it if you did not request the change."); e != nil {
		return &domain.Error{Kind: domain.Unavailable, Detail: "Verification email could not be sent. Your current email is unchanged. Please retry."}
	}
	return nil
}
func (s *AccountService) ConfirmEmail(ctx context.Context, raw string) error {
	invalid := &domain.Error{Kind: domain.Invalid, Detail: "This verification link is invalid or expired. Request a new link."}
	if len(raw) != 64 {
		return invalid
	}
	return s.unit.Within(ctx, func(ctx context.Context, r ports.AccountRepository) error {
		// Lock administration before the user to match administrative identity changes.
		if e := r.Lock(ctx, "auth.administration"); e != nil {
			return e
		}
		c, e := r.EmailChange(ctx, s.tokens.HashRefresh(raw))
		if e != nil {
			return e
		}
		if c == nil || c.Consumed || c.ExpiresAt.Unix() <= s.tokens.Now() {
			return invalid
		}
		u, e := r.User(ctx, c.UserID, true)
		if e != nil {
			return e
		}
		if u == nil || !u.IsActive || u.CredentialVersion != c.CredentialVersion || u.Email != c.PreviousEmail {
			return invalid
		}
		if e = r.SetActor(ctx, c.ActorID); e != nil {
			return e
		}
		if _, e = r.UpdateUser(ctx, u.ID, domain.UserUpdate{Email: &c.Email}, c.ActorID); e != nil {
			return e
		}
		if e = r.CancelEmailChanges(ctx, u.ID); e != nil {
			return e
		}
		if e = r.RevokeUserSessions(ctx, u.ID); e != nil {
			return e
		}
		return r.RecordAccountChange(ctx, s.tokens.ID(), u.ID, "email_verified")
	})
}
