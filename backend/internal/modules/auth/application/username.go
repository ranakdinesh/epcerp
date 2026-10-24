package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/ports"
)

func (s *AccountService) UsernameAvailable(ctx context.Context, token, name string) (bool, error) {
	name, e := domain.Username(name)
	if e != nil {
		return false, e
	}
	var available bool
	e = authorized(ctx, s.unit, s.tokens, token, "account.profile.write", false, func(ctx context.Context, r ports.AccountRepository, u domain.User) error {
		var err error
		available, err = r.UsernameAvailable(ctx, u.ID, name)
		return err
	})
	return available, e
}
func (s *AccountService) ChangeUsername(ctx context.Context, token, name, expected string) error {
	name, e := domain.Username(name)
	if e != nil {
		return e
	}
	return authorized(ctx, s.unit, s.tokens, token, "account.profile.write", true, func(ctx context.Context, r ports.AccountRepository, u domain.User) error {
		user, e := r.User(ctx, u.ID, true)
		if e != nil {
			return e
		}
		if user == nil || !user.IsActive || user.CredentialVersion != u.CredentialVersion || user.MustChangePassword {
			return domain.ErrUnauthorized
		}
		if user.Username != expected {
			return domain.ErrUsernameStale
		}
		if user.Username == name {
			return nil
		}
		available, e := r.UsernameAvailable(ctx, u.ID, name)
		if e != nil {
			return e
		}
		if !available {
			return domain.UsernameTaken(name)
		}
		return r.SaveUsername(ctx, s.tokens.ID(), u.ID, user.Username, name)
	})
}
