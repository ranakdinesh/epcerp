package infrastructure

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/identity"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
)

// AuthBridge translates the platform identity into shared identity values.
type AuthBridge struct{ resolver auth.Resolver }

func NewAuthBridge(r auth.Resolver) *AuthBridge { return &AuthBridge{r} }
func (b *AuthBridge) Resolve(ctx context.Context, token string) (identity.Principal, error) {
	p, e := b.resolver.Resolve(ctx, token)
	return identity.Principal{ID: p.ID, Active: p.Active, Superuser: p.Superuser, Permissions: p.Permissions}, e
}
