package application

import (
	"context"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/ports"
)

// AccountService owns self-service identity changes, separately from administration.
type AccountService struct {
	unit      ports.UnitOfWork[ports.AccountRepository]
	passwords ports.Passwords
	tokens    ports.Tokens
}

func NewAccount(u ports.UnitOfWork[ports.AccountRepository], p ports.Passwords, t ports.Tokens) *AccountService {
	return &AccountService{u, p, t}
}
func (s *AccountService) Change(ctx context.Context, token, current string, email, password *string) error {
	if (email == nil) == (password == nil) || len(current) > 512 {
		return &domain.Error{Kind: domain.Invalid, Detail: "One account change is required"}
	}
	if password != nil && !domain.StrongPassword(*password) {
		return &domain.Error{Kind: domain.Invalid, Detail: "Use a strong password of 8 to 128 characters"}
	}
	return s.unit.Within(ctx, func(ctx context.Context, r ports.AccountRepository) error {
		id, e := s.tokens.Subject(token)
		if e != nil {
			return domain.ErrUnauthorized
		}
		user, e := r.User(ctx, id, true)
		if e != nil {
			return e
		}
		version, versionError := s.tokens.Version(token)
		if versionError != nil || user == nil || !user.IsActive || user.CredentialVersion != version || !user.TemporaryValid(s.tokens.Now()) {
			return domain.ErrUnauthorized
		}
		if (user.MustChangePassword && password == nil) || (!user.MustChangePassword && !domain.HasPermission(*user, "account.security.write")) {
			return domain.ErrForbidden
		}
		if !s.passwords.Verify(ctx, current, user.PasswordHash) {
			return &domain.Error{Kind: domain.Invalid, Detail: "Current password is incorrect"}
		}
		action := "email_changed"
		if password != nil {
			if *password == current {
				return &domain.Error{Kind: domain.Invalid, Detail: "Choose a different password"}
			}
			hash, e := s.passwords.Hash(ctx, *password)
			if e != nil {
				return e
			}
			if e = r.SetPassword(ctx, id, hash, id); e != nil {
				return e
			}
			if e = r.RevokeUserSessions(ctx, id); e != nil {
				return e
			}
			action = "password_changed"
		} else if email != nil {
			if _, e = r.UpdateUser(ctx, id, domain.UserUpdate{Email: email}, id); e != nil {
				return e
			}
		} else {
			return &domain.Error{Kind: domain.Invalid, Detail: "A change is required"}
		}
		return r.RecordAccountChange(ctx, s.tokens.ID(), id, action)
	})
}
