package application

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type DispatchService struct {
	u ports.DispatchUnitOfWork
	a ports.DispatchAuthorizer
	v ports.DispatchValues
}

func NewDispatch(u ports.DispatchUnitOfWork, a ports.DispatchAuthorizer, v ports.DispatchValues) *DispatchService {
	return &DispatchService{u, a, v}
}
func (s *DispatchService) actor(ctx context.Context, token, p, id string) (domain.Principal, error) {
	a, e := s.a.Resolve(ctx, token)
	if e != nil || !a.Active {
		return a, domain.DispatchUnauthorized
	}
	for _, v := range []string{a.ID, p, id} {
		if e = domain.DispatchID(v); e != nil {
			return a, e
		}
	}
	return a, nil
}
func (s *DispatchService) workspace(ctx context.Context, r ports.DispatchRepository, c ports.DispatchContext, a domain.Principal, p, id string) (out domain.DispatchWorkspace, e error) {
	caps, e := c.Access(ctx, a, p, id)
	if e != nil {
		return out, e
	}
	if e = domain.DispatchAccessGuard(a, caps, false); e != nil {
		return out, e
	}
	out.Source, e = c.Source(ctx, p, id)
	if e != nil {
		return out, e
	}
	if out.Source.ProjectID != p || out.Source.PackageID != id {
		return out, domain.DispatchInvalid("source", "Dispatch source does not belong to this project/package")
	}
	out.Record, e = r.Get(ctx, p, id)
	if e != nil {
		return out, e
	}
	out.Actions = []string{}
	out.History = []domain.DispatchEvent{}
	out.TaskOnly = !caps.Scoped
	if out.Record != nil {
		v := domain.DispatchCopy(*out.Record)
		out.Record = &v
		if !out.Record.Active {
			return out, domain.DispatchMissing
		}
		if caps.Scoped {
			out.History, e = r.History(ctx, v.ID)
			if e != nil {
				return out, e
			}
		}
	}
	for _, action := range []string{"record", "revise"} {
		if domain.DispatchCan(out.Record, a, caps, action, out.Source) == nil {
			out.Actions = append(out.Actions, action)
		}
	}
	return out, nil
}
func (s *DispatchService) Workspace(ctx context.Context, token, p, id string) (out domain.DispatchWorkspace, e error) {
	a, e := s.actor(ctx, token, p, id)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.DispatchRepository, c ports.DispatchContext) error {
		out, e = s.workspace(ctx, r, c, a, p, id)
		return e
	})
	if e != nil {
		out = domain.DispatchWorkspace{}
	}
	return
}
func (s *DispatchService) Execute(ctx context.Context, token, p, id string, q domain.DispatchCommand) (out domain.DispatchWorkspace, e error) {
	a, e := s.actor(ctx, token, p, id)
	if e != nil {
		return out, e
	}
	q, e = domain.DispatchValidate(q)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.DispatchRepository, c ports.DispatchContext) error {
		caps, err := c.Access(ctx, a, p, id)
		if err != nil {
			return err
		}
		if err = domain.DispatchAccessGuard(a, caps, true); err != nil {
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
			return domain.DispatchStale
		}
		source, err := c.Source(ctx, p, id)
		if err != nil {
			return err
		}
		if source.ProjectID != p || source.PackageID != id {
			return domain.DispatchInvalid("source", "Dispatch source does not belong to this project/package")
		}
		if err = source.Guard(); err != nil {
			return err
		}
		now := s.v.Now()

		next, err := domain.DispatchApply(before, a, caps, q, source, now, s.v.ID())
		if err != nil {
			return err
		}
		saved, err := r.Save(ctx, next)
		if err != nil {
			return err
		}
		if err = r.Event(ctx, domain.DispatchEvent{ID: s.v.ID(), DispatchID: saved.ID, Action: q.Action, Reason: q.Reason, Before: before, After: saved, At: saved.UpdatedAt, ActorID: a.ID}); err != nil {
			return err
		}
		out, err = s.workspace(ctx, r, c, a, p, id)
		return err
	})
	if e != nil {
		out = domain.DispatchWorkspace{}
	}
	return
}
