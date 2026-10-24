package application

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type AcceptanceService struct {
	u ports.AcceptanceUnitOfWork
	a ports.AcceptanceAuthorizer
	v ports.AcceptanceValues
}

func NewAcceptance(u ports.AcceptanceUnitOfWork, a ports.AcceptanceAuthorizer, v ports.AcceptanceValues) *AcceptanceService {
	return &AcceptanceService{u, a, v}
}
func (s *AcceptanceService) actor(ctx context.Context, token, p, id string) (domain.Principal, error) {
	a, e := s.a.Resolve(ctx, token)
	if e != nil || !a.Active {
		return a, domain.AcceptanceUnauthorized
	}
	for _, v := range []string{a.ID, p, id} {
		if e = domain.AcceptanceID(v); e != nil {
			return a, e
		}
	}
	return a, nil
}
func (s *AcceptanceService) workspace(ctx context.Context, r ports.AcceptanceRepository, c ports.AcceptanceContext, a domain.Principal, p, id string) (out domain.AcceptanceWorkspace, e error) {
	caps, e := c.Access(ctx, a, p, id)
	if e != nil {
		return out, e
	}
	if e = domain.AcceptanceAccessGuard(a, caps, false); e != nil {
		return out, e
	}
	out.Source, e = c.Source(ctx, p, id)
	if e != nil {
		return out, e
	}
	if out.Source.ProjectID != p || out.Source.PackageID != id {
		return out, domain.AcceptanceInvalid("source", "Acceptance source does not belong to this project/package")
	}
	out.Record, e = r.Get(ctx, p, id)
	if e != nil {
		return out, e
	}
	out.Actions = []string{}
	out.History = []domain.AcceptanceEvent{}
	out.TaskOnly = !caps.Scoped
	if out.Record != nil {
		v := *out.Record
		out.Record = &v
		if !out.Record.Active {
			return out, domain.AcceptanceMissing
		}
		if caps.Scoped {
			out.History, e = r.History(ctx, v.ID)
			if e != nil {
				return out, e
			}
		}
	}
	if out.Source.Guard() == nil && domain.AcceptanceAccessGuard(a, caps, true) == nil {
		out.Actions = append(out.Actions, "record")
	}
	return out, nil
}
func (s *AcceptanceService) Workspace(ctx context.Context, token, p, id string) (out domain.AcceptanceWorkspace, e error) {
	a, e := s.actor(ctx, token, p, id)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.AcceptanceRepository, c ports.AcceptanceContext) error {
		out, e = s.workspace(ctx, r, c, a, p, id)
		return e
	})
	if e != nil {
		out = domain.AcceptanceWorkspace{}
	}
	return
}
func (s *AcceptanceService) Execute(ctx context.Context, token, p, id string, q domain.AcceptanceCommand) (out domain.AcceptanceWorkspace, e error) {
	a, e := s.actor(ctx, token, p, id)
	if e != nil {
		return out, e
	}
	q, e = domain.AcceptanceValidate(q)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.AcceptanceRepository, c ports.AcceptanceContext) error {
		caps, err := c.Access(ctx, a, p, id)
		if err != nil {
			return err
		}
		if err = domain.AcceptanceAccessGuard(a, caps, true); err != nil {
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
			return domain.AcceptanceStale
		}
		source, err := c.Source(ctx, p, id)
		if err != nil {
			return err
		}
		if source.ProjectID != p || source.PackageID != id {
			return domain.AcceptanceInvalid("source", "Acceptance source does not belong to this project/package")
		}
		if err = source.Guard(); err != nil {
			return err
		}
		now := s.v.Now()

		next, err := domain.AcceptanceApply(before, a, caps, q, source, now, s.v.ID())
		if err != nil {
			return err
		}
		saved, err := r.Save(ctx, next)
		if err != nil {
			return err
		}
		if err = r.Event(ctx, domain.AcceptanceEvent{ID: s.v.ID(), AcceptanceID: saved.ID, Action: q.Action, Reason: q.Reason, Before: before, After: saved, At: saved.UpdatedAt, ActorID: a.ID}); err != nil {
			return err
		}
		out, err = s.workspace(ctx, r, c, a, p, id)
		return err
	})
	if e != nil {
		out = domain.AcceptanceWorkspace{}
	}
	return
}
