package outbound

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/storage/domain"
)

type AuthBridge struct{ resolver contracts.Resolver }

func NewAuthBridge(resolver contracts.Resolver) *AuthBridge { return &AuthBridge{resolver} }
func (b *AuthBridge) Resolve(ctx context.Context, token string) (domain.Principal, error) {
	identity, err := b.resolver.Resolve(ctx, token)
	if err != nil {
		return domain.Principal{}, err
	}
	return domain.Principal{ID: identity.ID, Active: identity.Active, Superuser: identity.Superuser, Permissions: identity.Permissions}, nil
}
