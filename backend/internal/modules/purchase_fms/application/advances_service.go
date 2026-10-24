package application

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type AdvancesService struct {
	u ports.AdvancesUnitOfWork
	a ports.AdvancesAuthorizer
	v ports.AdvancesValues
}

func NewAdvances(u ports.AdvancesUnitOfWork, a ports.AdvancesAuthorizer, v ports.AdvancesValues) *AdvancesService {
	return &AdvancesService{u, a, v}
}
func (s *AdvancesService) actor(ctx context.Context, token, p, id string) (domain.Principal, error) {
	a, e := s.a.Resolve(ctx, token)
	if e != nil || !a.Active {
		return a, domain.AdvancesUnauthorized
	}
	for _, v := range []string{a.ID, p, id} {
		if e = domain.AdvancesID(v); e != nil {
			return a, e
		}
	}
	return a, nil
}
func (s *AdvancesService) workspace(ctx context.Context, r ports.AdvancesRepository, c ports.AdvancesContext, a domain.Principal, p, id string) (out domain.AdvancesWorkspace, e error) {
	caps, e := c.Access(ctx, a, p, id)
	if e != nil {
		return out, e
	}
	if e = domain.AdvancesAccessGuard(a, caps, "read"); e != nil {
		return out, e
	}
	out.Source, e = c.Source(ctx, p, id)
	if e != nil {
		return out, e
	}
	if out.Source.ProjectID != p || out.Source.PackageID != id {
		return out, domain.AdvancesInvalid("source", "Advance source belongs to another project/package")
	}
	out.Record, e = r.Get(ctx, p, id)
	if e != nil {
		return out, e
	}
	out.Actions = []string{}
	out.History = []domain.AdvancesEvent{}
	out.TaskOnly = !caps.Scoped
	if out.Record != nil {
		v := domain.AdvancesCopy(*out.Record)
		out.Record = &v
		if !v.Active {
			return out, domain.AdvancesMissing
		}
		if caps.Scoped {
			out.History, e = r.History(ctx, v.ID)
			if e != nil {
				return out, e
			}
		}
	}
	for _, action := range []string{"prepare", "submit", "revise", "approve", "reject", "disburse"} {
		if domain.AdvancesCan(out.Record, a, caps, action, out.Source) == nil {
			out.Actions = append(out.Actions, action)
		}
	}
	return out, nil
}
func (s *AdvancesService) Workspace(ctx context.Context, token, p, id string) (out domain.AdvancesWorkspace, e error) {
	a, e := s.actor(ctx, token, p, id)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.AdvancesRepository, c ports.AdvancesContext, f ports.AdvancesFinance) error {
		out, e = s.workspace(ctx, r, c, a, p, id)
		return e
	})
	if e != nil {
		out = domain.AdvancesWorkspace{}
	}
	return
}
func (s *AdvancesService) Execute(ctx context.Context, token, p, id string, q domain.AdvancesCommand) (out domain.AdvancesWorkspace, e error) {
	a, e := s.actor(ctx, token, p, id)
	if e != nil {
		return out, e
	}
	q, e = domain.AdvancesValidate(q)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.AdvancesRepository, c ports.AdvancesContext, f ports.AdvancesFinance) error {
		caps, err := c.Access(ctx, a, p, id)
		if err != nil {
			return err
		}
		if err = domain.AdvancesAccessGuard(a, caps, q.Action); err != nil {
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
			return domain.AdvancesStale
		}
		source, err := c.Source(ctx, p, id)
		if err != nil {
			return err
		}
		if source.ProjectID != p || source.PackageID != id {
			return domain.AdvancesInvalid("source", "Advance source belongs to another project/package")
		}
		if err = domain.AdvancesCan(before, a, caps, q.Action, source); err != nil {
			return err
		}
		now := s.v.Now()
		decision := ""
		if q.Action == "approve" || q.Action == "reject" {
			if err = domain.AdvancesFinanceReady(*before, source, domain.AdvancesToday(now)); err != nil {
				return err
			}
			outcome := "approved"
			if q.Action == "reject" {
				outcome = "rejected"
			}
			decision, err = f.Decide(ctx, a, domain.AdvancesCopy(*before), outcome, q.Reason)
			if err != nil {
				return err
			}
		}
		next, err := domain.AdvancesApply(before, a, caps, q, source, now, s.v.ID(), decision)
		if err != nil {
			return err
		}
		saved, err := r.Save(ctx, next)
		if err != nil {
			return err
		}
		if err = r.Event(ctx, domain.AdvancesEvent{ID: s.v.ID(), RequestID: saved.ID, Action: q.Action, Reason: q.Reason, Before: before, After: saved, At: saved.UpdatedAt, ActorID: a.ID}); err != nil {
			return err
		}
		out, err = s.workspace(ctx, r, c, a, p, id)
		return err
	})
	if e != nil {
		out = domain.AdvancesWorkspace{}
	}
	return
}
