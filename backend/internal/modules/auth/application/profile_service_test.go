package application

import (
	"context"
	"errors"
	"testing"

	"github.com/ranakdinesh/epcERP/internal/identity"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

type profileIdentity struct {
	principal identity.Principal
	err       error
}

func (i profileIdentity) Resolve(context.Context, string) (identity.Principal, error) {
	return i.principal, i.err
}

func TestProfileRejectsInvalidIdentityBeforePersistence(t *testing.T) {
	resolverFailure := errors.New("identity service unavailable")
	for _, test := range []struct {
		name          string
		principal     identity.Principal
		failure, want error
	}{
		{name: "missing actor", principal: identity.Principal{Active: true, Superuser: true}, want: domain.ErrUnauthorized},
		{name: "inactive recovery actor", principal: identity.Principal{ID: "actor", Superuser: true}, want: domain.ErrUnauthorized},
		{name: "missing live permission", principal: identity.Principal{ID: "actor", Active: true}, want: domain.ErrForbidden},
		{name: "resolver failure", failure: resolverFailure, want: resolverFailure},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Nil persistence dependencies prove both reads and writes stop at authorization.
			service := NewProfile(nil, profileIdentity{test.principal, test.failure}, nil, nil, nil)
			if _, err := service.Get(context.Background(), "token", ""); !errors.Is(err, test.want) {
				t.Fatalf("read: got %v want %v", err, test.want)
			}
			if _, err := service.Update(context.Background(), "token", nil, nil, false); !errors.Is(err, test.want) {
				t.Fatalf("write: got %v want %v", err, test.want)
			}
		})
	}
}
