package infrastructure

import (
	"context"
	"errors"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	"testing"
)

type identityResolverFixture struct {
	value auth.Identity
	err   error
}

func (f identityResolverFixture) Resolve(context.Context, string) (auth.Identity, error) {
	return f.value, f.err
}

func TestSharedAuthBridgePreservesIdentityAndResolverFailure(t *testing.T) {
	resolved := auth.Identity{ID: "actor", Active: true, Superuser: true, Permissions: []string{"settings.clients.read"}}
	principal, err := NewAuthBridge(identityResolverFixture{value: resolved}).Resolve(context.Background(), "fixture")
	if err != nil || principal.ID != resolved.ID || principal.Active != resolved.Active || principal.Superuser != resolved.Superuser || len(principal.Permissions) != 1 || principal.Permissions[0] != resolved.Permissions[0] {
		t.Fatal("identity mapping changed")
	}
	failure := errors.New("resolver unavailable")
	principal, err = NewAuthBridge(identityResolverFixture{err: failure}).Resolve(context.Background(), "fixture")
	if !errors.Is(err, failure) || principal.Allows("settings.clients.read") {
		t.Fatal("resolver failure must not grant authority")
	}
	principal, err = NewAuthBridge(identityResolverFixture{}).Resolve(context.Background(), "fixture")
	if err != nil || principal.Allows("settings.clients.read") {
		t.Fatal("invalid identity must not grant authority")
	}
}
