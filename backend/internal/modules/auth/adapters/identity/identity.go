package identity

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

type IdentityResolver struct{ service *application.SessionService }

func NewIdentityResolver(service *application.SessionService) *IdentityResolver {
	return &IdentityResolver{service}
}

var _ contracts.Resolver = (*IdentityResolver)(nil)

func (r *IdentityResolver) Resolve(ctx context.Context, token string) (contracts.Identity, error) {
	user, err := r.service.Me(ctx, token)
	if errors.Is(err, domain.ErrUnauthorized) {
		return contracts.Identity{}, nil
	}
	if err != nil {
		return contracts.Identity{}, err
	}
	return contracts.Identity{AccountNotReady: user.IsActive && user.MustChangePassword, ID: user.ID, Active: user.IsActive && !user.MustChangePassword, Superuser: user.IsSuperuser && !user.MustChangePassword, Permissions: domain.EffectivePermissions(*user)}, nil
}
