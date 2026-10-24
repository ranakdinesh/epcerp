package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/ports"
)

type Service struct {
	u ports.UnitOfWork
	a ports.Authorizer
	v ports.Values
}

func New(u ports.UnitOfWork, a ports.Authorizer, v ports.Values) *Service { return &Service{u, a, v} }
func (s *Service) actor(ctx context.Context, t string) (domain.Actor, error) {
	a, e := s.a.Resolve(ctx, t)
	if e != nil {
		return a, e
	}
	if !a.Active || a.ID == "" {
		return a, domain.Unauthorized
	}
	if !a.Can("admin.governance.read") {
		return a, domain.Forbidden
	}
	return a, nil
}
func live(ctx context.Context, users ports.Users, a domain.Actor, write bool) (domain.Actor, error) {
	u, e := users.Get(ctx, a.ID)
	if e != nil {
		return a, e
	}
	if u == nil || !u.Active {
		return a, domain.Unauthorized
	}
	a = u.Actor()
	if !a.Can("admin.governance.read") || (write && !a.Can("admin.governance.write")) {
		return a, domain.Forbidden
	}
	return a, nil
}
func workspace(ctx context.Context, r ports.Repository, users ports.Users, a domain.Actor) (domain.Workspace, error) {
	v, e := r.Get(ctx)
	if e != nil {
		return domain.Workspace{}, e
	}
	out := domain.Workspace{Record: v, CanWrite: a.Can("admin.governance.write"), History: []domain.Event{}}
	if v != nil {
		out.User, e = users.Get(ctx, v.UserID)
		if e != nil {
			return domain.Workspace{}, e
		}
		out.Effective = v.Active && out.User != nil && out.User.Active
	}
	out.History, e = r.History(ctx)
	return out, e
}
func (s *Service) Workspace(ctx context.Context, t string) (out domain.Workspace, e error) {
	a, e := s.actor(ctx, t)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository, u ports.Users) error {
		if e := r.Lock(ctx); e != nil {
			return e
		}
		a, e = live(ctx, u, a, false)
		if e != nil {
			return e
		}
		out, e = workspace(ctx, r, u, a)
		return e
	})
	if e != nil {
		out = domain.Workspace{}
	}
	return
}
func (s *Service) Options(ctx context.Context, t string) (out []domain.User, e error) {
	a, e := s.actor(ctx, t)
	if e != nil {
		return nil, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository, u ports.Users) error {
		if e := r.Lock(ctx); e != nil {
			return e
		}
		if _, e = live(ctx, u, a, true); e != nil {
			return e
		}
		out, e = u.List(ctx)
		return e
	})
	if e != nil {
		out = nil
	}
	return
}
func (s *Service) Save(ctx context.Context, t string, input domain.Input) (out domain.Workspace, e error) {
	a, e := s.actor(ctx, t)
	if e != nil {
		return out, e
	}
	input, e = input.Validate()
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository, u ports.Users) error {
		if e := r.Lock(ctx); e != nil {
			return e
		}
		a, e = live(ctx, u, a, true)
		if e != nil {
			return e
		}
		before, e := r.Get(ctx)
		if e != nil {
			return e
		}
		candidate, e := u.Get(ctx, input.UserID)
		if e != nil {
			return e
		}
		if candidate == nil || (input.Active && !candidate.Active) {
			return domain.Invalid("user_id", "Choose an active account for Director designation")
		}
		if !input.Active && (before == nil || before.UserID != input.UserID) {
			return domain.Invalid("user_id", "Retain the current Director when revoking the designation")
		}
		next, action, e := domain.Change(before, input, s.v.ID(), a.ID, s.v.Now())
		if e != nil {
			return e
		}
		saved, e := r.Save(ctx, next)
		if e != nil {
			return e
		}
		if e = r.Event(ctx, domain.Event{ID: s.v.ID(), Action: action, Reason: input.Reason, Before: before, After: saved, ActorID: a.ID, At: saved.UpdatedAt}); e != nil {
			return e
		}
		out, e = workspace(ctx, r, u, a)
		return e
	})
	if e != nil {
		out = domain.Workspace{}
	}
	return
}

// Recipient resolves only effective designation. Consumers authorize their own use case.
func Recipient(ctx context.Context, r ports.Repository, u ports.Users) (*domain.Designation, error) {
	if e := r.Lock(ctx); e != nil {
		return nil, e
	}
	v, e := r.Get(ctx)
	if e != nil || v == nil || !v.Active {
		return nil, e
	}
	user, e := u.Get(ctx, v.UserID)
	if e != nil || user == nil || !user.Active {
		return nil, e
	}
	return v, nil
}
