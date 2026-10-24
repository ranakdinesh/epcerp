package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/ports"
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
	if !a.Can("communication.inbox.read") {
		return a, domain.Forbidden
	}
	return a, nil
}
func live(ctx context.Context, u ports.Users, a domain.Actor, write bool) (domain.Actor, error) {
	v, e := u.Get(ctx, a.ID)
	if e != nil {
		return a, e
	}
	if v == nil || !v.Active {
		return a, domain.Unauthorized
	}
	a = *v
	if !a.Can("communication.inbox.read") || write && !a.Can("communication.inbox.write") {
		return a, domain.Forbidden
	}
	return a, nil
}
func (s *Service) List(ctx context.Context, t string, q domain.Query) (out domain.Listing, e error) {
	a, e := s.actor(ctx, t)
	if e != nil {
		return out, e
	}
	q, e = q.Validate()
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository, u ports.Users) error {
		a, e = live(ctx, u, a, false)
		if e != nil {
			return e
		}
		out = domain.Listing{Page: q.Page, Size: q.Size, CanWrite: a.Can("communication.inbox.write")}
		out.Items, out.Total, e = r.List(ctx, a.ID, q)
		return e
	})
	if e != nil {
		out = domain.Listing{}
	}
	return
}
func workspace(ctx context.Context, r ports.Repository, a domain.Actor, id string) (domain.Workspace, error) {
	n, e := r.Get(ctx, id, a.ID)
	if e != nil {
		return domain.Workspace{}, e
	}
	if n == nil {
		return domain.Workspace{}, domain.Missing
	}
	history, e := r.History(ctx, id, a.ID)
	return domain.Workspace{Notice: *n, History: history, CanWrite: a.Can("communication.inbox.write")}, e
}
func (s *Service) Read(ctx context.Context, t, id string) (out domain.Workspace, e error) {
	a, e := s.actor(ctx, t)
	if e != nil {
		return out, e
	}
	id, e = domain.ID(id)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository, u ports.Users) error {
		a, e = live(ctx, u, a, false)
		if e != nil {
			return e
		}
		out, e = workspace(ctx, r, a, id)
		return e
	})
	if e != nil {
		out = domain.Workspace{}
	}
	return
}
func (s *Service) Receipt(ctx context.Context, t, id string, version int32, action string) (out domain.Workspace, e error) {
	a, e := s.actor(ctx, t)
	if e != nil {
		return out, e
	}
	id, e = domain.ID(id)
	if e != nil {
		return out, e
	}
	if version < 1 {
		return out, domain.Invalid("version", "A current notice version is required")
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository, u ports.Users) error {
		a, e = live(ctx, u, a, true)
		if e != nil {
			return e
		}
		before, e := r.Get(ctx, id, a.ID)
		if e != nil {
			return e
		}
		if before == nil {
			return domain.Missing
		}
		next, changed, e := domain.Receipt(*before, a.ID, version, action, s.v.Now())
		if e != nil {
			return e
		}
		if changed {
			next, e = r.Save(ctx, next)
			if e != nil {
				return e
			}
			if e = r.Event(ctx, domain.Event{ID: s.v.ID(), Action: action, Before: before, After: next, ActorID: a.ID, At: next.UpdatedAt}); e != nil {
				return e
			}
		}
		out, e = workspace(ctx, r, a, id)
		return e
	})
	if e != nil {
		out = domain.Workspace{}
	}
	return
}

// Deliver is called only through a transaction-bound published adapter.
func Deliver(ctx context.Context, r ports.Repository, d ports.DirectorDirectory, v ports.Values, eventID string) (domain.Notice, error) {
	eventID, e := domain.ID(eventID)
	if e != nil {
		return domain.Notice{}, e
	}
	source, e := r.Source(ctx, eventID)
	if e != nil {
		return domain.Notice{}, e
	}
	if source == nil {
		return domain.Notice{}, domain.SourceConflict
	}
	if e = source.Validate(); e != nil {
		return domain.Notice{}, e
	}
	existing, e := r.BySource(ctx, eventID)
	if e != nil {
		return domain.Notice{}, e
	}
	if existing != nil {
		if !existing.Matches(*source) {
			return domain.Notice{}, domain.SourceConflict
		}
		return *existing, nil
	}
	director, e := d.Current(ctx)
	if e != nil {
		return domain.Notice{}, e
	}
	if director == nil {
		return domain.Notice{}, domain.NoDirector
	}
	n, e := domain.New(v.ID(), *source, *director)
	if e != nil {
		return domain.Notice{}, e
	}
	saved, e := r.Create(ctx, n)
	if e != nil {
		return domain.Notice{}, e
	}
	if !saved.Matches(*source) {
		return domain.Notice{}, domain.SourceConflict
	}
	if saved.ID != n.ID {
		return saved, nil
	}
	e = r.Event(ctx, domain.Event{ID: v.ID(), Action: "delivered", After: saved, ActorID: source.ActorID, At: saved.CreatedAt})
	if e != nil {
		return domain.Notice{}, e
	}
	return saved, nil
}
