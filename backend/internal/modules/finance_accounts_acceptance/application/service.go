package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/ports"
)

type Service struct {
	u ports.UnitOfWork
	a ports.Authorizer
	v ports.Values
}

func New(u ports.UnitOfWork, a ports.Authorizer, v ports.Values) *Service { return &Service{u, a, v} }
func (s *Service) actor(ctx context.Context, token, p, id string) (domain.Actor, error) {
	a, e := s.a.Resolve(ctx, token)
	if e != nil || !a.Active {
		return a, domain.Unauthorized
	}
	for _, v := range []string{a.ID, p, id} {
		if e = domain.ID(v); e != nil {
			return a, e
		}
	}
	return a, nil
}
func (s *Service) workspace(ctx context.Context, r ports.Repository, c ports.Context, a domain.Actor, p, id string) (out domain.Workspace, e error) {
	caps, e := c.Access(ctx, a, p, id)
	if e != nil {
		return out, e
	}
	if e = domain.AccessGuard(a, caps, false); e != nil {
		return out, e
	}
	out.Source, e = c.Source(ctx, p, id)
	if e != nil {
		return out, e
	}
	if out.Source.ProjectID != p || out.Source.PackageID != id {
		return out, domain.Invalid("source", "Acceptance source does not belong to this project/package")
	}
	out.Record, e = r.Get(ctx, p, id)
	if e != nil {
		return out, e
	}
	out.Actions = []string{}
	out.History = []domain.Event{}
	out.TaskOnly = !caps.Scoped
	if out.Record != nil {
		v := domain.Copy(*out.Record)
		out.Record = &v
		if !out.Record.Active {
			return out, domain.Missing
		}
		if caps.Scoped {
			out.History, e = r.History(ctx, v.ID)
			if e != nil {
				return out, e
			}
		}
	}
	for _, action := range []string{"record", "revise"} {
		if domain.Can(out.Record, a, caps, action, out.Source) == nil {
			out.Actions = append(out.Actions, action)
		}
	}
	return out, nil
}
func (s *Service) Workspace(ctx context.Context, token, p, id string) (out domain.Workspace, e error) {
	a, e := s.actor(ctx, token, p, id)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository, c ports.Context) error {
		out, e = s.workspace(ctx, r, c, a, p, id)
		return e
	})
	if e != nil {
		out = domain.Workspace{}
	}
	return
}
func (s *Service) Execute(ctx context.Context, token, p, id string, q domain.Command) (out domain.Workspace, e error) {
	a, e := s.actor(ctx, token, p, id)
	if e != nil {
		return out, e
	}
	q, e = domain.Validate(q)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository, c ports.Context) error {
		caps, err := c.Access(ctx, a, p, id)
		if err != nil {
			return err
		}
		if err = domain.AccessGuard(a, caps, true); err != nil {
			return err
		}
		before, err := r.Get(ctx, p, id)
		if err != nil {
			return err
		}
		version := int32(0)
		if before != nil {
			version = before.Version
		}
		if q.Version != version {
			return domain.Stale
		}
		source, err := c.Source(ctx, p, id)
		if err != nil {
			return err
		}
		if source.ProjectID != p || source.PackageID != id {
			return domain.Invalid("source", "Acceptance source does not belong to this project/package")
		}
		if err = source.Guard(); err != nil {
			return err
		}
		now := s.v.Now()

		next, err := domain.Apply(before, a, caps, q, source, now, s.v.ID())
		if err != nil {
			return err
		}
		saved, err := r.Save(ctx, next)
		if err != nil {
			return err
		}
		event := domain.Event{ID: s.v.ID(), AcceptanceID: saved.ID, Action: q.Action, Reason: q.Reason, Before: before, After: saved, At: saved.UpdatedAt, ActorID: a.ID}
		if err = r.Event(ctx, event); err != nil {
			return err
		}
		if domain.FinancialEvent(event) {
			if err = r.Director(ctx, event.ID); err != nil {
				return err
			}
		}
		out, err = s.workspace(ctx, r, c, a, p, id)
		return err
	})
	if e != nil {
		out = domain.Workspace{}
	}
	return
}
