package application

import (
	"context"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/ports"
	"testing"
)

type authStub struct{ p domain.Principal }

func (a authStub) Resolve(context.Context, string) (domain.Principal, error) { return a.p, nil }

type deniedUnit struct{ calls int }

func (u *deniedUnit) Within(context.Context, func(context.Context, ports.Repository, ports.Clients, ports.Users, ports.Calendar, ports.Purchasing) error) error {
	u.calls++
	return errors.New("unexpected persistence")
}

type ids struct{}

func (ids) ID() string { return "00000000-0000-0000-0000-000000000001" }
func TestDeniedActionsDoNotReachPersistence(t *testing.T) {
	ctx := context.Background()
	u := &deniedUnit{}
	for _, p := range []domain.Principal{
		{ID: "actor", Active: false, Superuser: true},
		{ID: "actor", Active: true, Permissions: []string{"projects.portfolio.write"}},
		{ID: "actor", Active: true, Permissions: []string{"projects.master.read", "projects.master.write"}},
	} {
		s := New(u, authStub{p}, ids{})
		if _, e := s.Create(ctx, "token", domain.Create{}); e == nil {
			t.Fatal("unauthorized create")
		}
		if _, e := s.Transfer(ctx, "token", ids{}.ID(), domain.Transfer{}); e == nil {
			t.Fatal("unauthorized handover")
		}
	}
	if u.calls != 0 {
		t.Fatal("authorization reached persistence")
	}
	s := New(u, authStub{domain.Principal{ID: "actor", Active: true, Superuser: true}}, ids{})
	if _, e := s.Create(ctx, "token", domain.Create{}); e == nil {
		t.Fatal("invalid input")
	}
	if _, e := s.Transfer(ctx, "token", ids{}.ID(), domain.Transfer{ManagerID: ids{}.ID(), Reason: " ", ExpectedVersion: 1}); e == nil {
		t.Fatal("missing reason")
	}
	if u.calls != 0 {
		t.Fatal("invalid input reached persistence")
	}
}
