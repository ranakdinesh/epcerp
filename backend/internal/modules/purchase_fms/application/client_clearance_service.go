package application

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type ClientClearanceService struct {
	u ports.ClientClearanceUnitOfWork
	a ports.ClientClearanceAuthorizer
	v ports.ClientClearanceValues
}

func NewClientClearance(u ports.ClientClearanceUnitOfWork, a ports.ClientClearanceAuthorizer, v ports.ClientClearanceValues) *ClientClearanceService {
	return &ClientClearanceService{u, a, v}
}
func (s *ClientClearanceService) actor(ctx context.Context, token, p, id string) (domain.Principal, error) {
	a, e := s.a.Resolve(ctx, token)
	if e != nil || !a.Active {
		return a, domain.ClientClearanceUnauthorized
	}
	for _, v := range []string{a.ID, p, id} {
		if e = domain.ClientClearanceID(v); e != nil {
			return a, e
		}
	}
	return a, nil
}
func (s *ClientClearanceService) workspace(ctx context.Context, r ports.ClientClearanceRepository, c ports.ClientClearanceContext, a domain.Principal, p, id string) (out domain.ClientClearanceWorkspace, e error) {
	caps, e := c.Access(ctx, a, p, id)
	if e != nil {
		return out, e
	}
	if e = domain.ClientClearanceAccessGuard(a, caps, false); e != nil {
		return out, e
	}
	out.Source, e = c.Source(ctx, p, id)
	if e != nil {
		return out, e
	}
	if out.Source.ProjectID != p || out.Source.PackageID != id {
		return out, domain.ClientClearanceInvalid("source", "Clearance source does not belong to this project/package")
	}
	out.Record, e = r.Get(ctx, p, id)
	if e != nil {
		return out, e
	}
	out.Actions = []string{}
	out.History = []domain.ClientClearanceEvent{}
	out.TaskOnly = !caps.Scoped
	if out.Record != nil {
		v := domain.ClientClearanceCopy(*out.Record)
		out.Record = &v
		if !out.Record.Active {
			return out, domain.ClientClearanceMissing
		}
		if caps.Scoped {
			out.History, e = r.History(ctx, v.ID)
			if e != nil {
				return out, e
			}
		}
	}
	for _, action := range []string{"record", "revise"} {
		if domain.ClientClearanceCan(out.Record, a, caps, action, out.Source) == nil {
			out.Actions = append(out.Actions, action)
		}
	}
	return out, nil
}
func (s *ClientClearanceService) Workspace(ctx context.Context, token, p, id string) (out domain.ClientClearanceWorkspace, e error) {
	a, e := s.actor(ctx, token, p, id)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.ClientClearanceRepository, c ports.ClientClearanceContext) error {
		out, e = s.workspace(ctx, r, c, a, p, id)
		return e
	})
	if e != nil {
		out = domain.ClientClearanceWorkspace{}
	}
	return
}
func (s *ClientClearanceService) Execute(ctx context.Context, token, p, id string, q domain.ClientClearanceCommand) (out domain.ClientClearanceWorkspace, e error) {
	a, e := s.actor(ctx, token, p, id)
	if e != nil {
		return out, e
	}
	q, e = domain.ClientClearanceValidate(q)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.ClientClearanceRepository, c ports.ClientClearanceContext) error {
		caps, err := c.Access(ctx, a, p, id)
		if err != nil {
			return err
		}
		if err = domain.ClientClearanceAccessGuard(a, caps, true); err != nil {
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
			return domain.ClientClearanceStale
		}
		source, err := c.Source(ctx, p, id)
		if err != nil {
			return err
		}
		if source.ProjectID != p || source.PackageID != id {
			return domain.ClientClearanceInvalid("source", "Clearance source does not belong to this project/package")
		}
		if err = source.Guard(); err != nil {
			return err
		}
		now := s.v.Now()

		next, err := domain.ClientClearanceApply(before, a, caps, q, source, now, s.v.ID())
		if err != nil {
			return err
		}
		saved, err := r.Save(ctx, next)
		if err != nil {
			return err
		}
		if err = r.Event(ctx, domain.ClientClearanceEvent{ID: s.v.ID(), ClearanceID: saved.ID, Action: q.Action, Reason: q.Reason, Before: before, After: saved, At: saved.UpdatedAt, ActorID: a.ID}); err != nil {
			return err
		}
		out, err = s.workspace(ctx, r, c, a, p, id)
		return err
	})
	if e != nil {
		out = domain.ClientClearanceWorkspace{}
	}
	return
}
