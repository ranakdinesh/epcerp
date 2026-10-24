package application

import (
	"context"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/ports"
)

func resolveSelf(ctx context.Context, repo ports.IdentityReader, tokens ports.Tokens, token string) (*domain.User, error) {
	id, err := tokens.Subject(token)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}
	user, err := repo.User(ctx, id, false)
	if err != nil {
		return nil, err
	}
	version, e := tokens.Version(token)
	if e != nil || user == nil || !user.IsActive || user.CredentialVersion != version || !user.TemporaryValid(tokens.Now()) {
		return nil, domain.ErrUnauthorized
	}
	return user, nil
}

func resolve(ctx context.Context, repo ports.IdentityReader, tokens ports.Tokens, token string) (*domain.User, error) {
	user, e := resolveSelf(ctx, repo, tokens, token)
	if e != nil {
		return nil, e
	}
	if user.MustChangePassword {
		return nil, domain.ErrForbidden
	}
	return user, nil
}

func authorized[R ports.AdministrationRepository](ctx context.Context, unit ports.UnitOfWork[R], tokens ports.Tokens, token, permission string, write bool, work func(context.Context, R, domain.User) error) error {
	return unit.Within(ctx, func(ctx context.Context, repo R) error {
		if write {
			if err := repo.Lock(ctx, "auth.administration"); err != nil {
				return err
			}
		}
		actor, err := resolve(ctx, repo, tokens, token)
		if err != nil {
			return err
		}
		if !domain.HasPermission(*actor, permission) {
			return domain.ErrForbidden
		}
		return work(ctx, repo, *actor)
	})
}
