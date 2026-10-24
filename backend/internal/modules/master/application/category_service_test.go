package application

import (
	"context"
	sharedIdentity "github.com/ranakdinesh/epcERP/internal/identity"
	"testing"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

type categoryFakeAuth struct {
	p sharedIdentity.Principal
}

func (a categoryFakeAuth) Resolve(context.Context, string) (sharedIdentity.Principal, error) {
	return a.p, nil
}

type categoryForbiddenUnit struct{ called bool }

func (u *categoryForbiddenUnit) Within(context.Context, func(context.Context, masterPorts.CategoryRepository) error) error {
	u.called = true
	return nil
}

type categoryValues struct{}

func (categoryValues) ID() string                                     { return "fixture" }
func (categoryValues) Fingerprint(masterDomain.CategorySource) string { return "fixture" }
func TestCategoryUnauthorizedAndInvalidImportsNeverOpenTransaction(t *testing.T) {
	for _, p := range []sharedIdentity.Principal{{}, {ID: "actor", Active: true, Permissions: []string{"settings.categories.read"}}, {ID: "actor", Active: true, Superuser: true}} {
		u := &categoryForbiddenUnit{}
		s := NewCategoryService(u, categoryFakeAuth{p}, categoryValues{})
		if _, e := s.Import(context.Background(), "fixture", nil); e == nil {
			t.Fatal("invalid import accepted")
		}
		if u.called {
			t.Fatal("transaction opened before authorization/validation")
		}
	}
}
