package application

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type SiteReceiptsService struct {
	u ports.SiteReceiptsUnitOfWork
	a ports.SiteReceiptsAuthorizer
	v ports.SiteReceiptsValues
}

func NewSiteReceipts(u ports.SiteReceiptsUnitOfWork, a ports.SiteReceiptsAuthorizer, v ports.SiteReceiptsValues) *SiteReceiptsService {
	return &SiteReceiptsService{u, a, v}
}
func (s *SiteReceiptsService) actor(ctx context.Context, token, p, id string) (domain.Principal, error) {
	a, e := s.a.Resolve(ctx, token)
	if e != nil || !a.Active {
		return a, domain.SiteReceiptsUnauthorized
	}
	for _, v := range []string{a.ID, p, id} {
		if e = domain.SiteReceiptsID(v); e != nil {
			return a, e
		}
	}
	return a, nil
}
func (s *SiteReceiptsService) workspace(ctx context.Context, r ports.SiteReceiptsRepository, c ports.SiteReceiptsContext, a domain.Principal, p, id string) (out domain.SiteReceiptsWorkspace, e error) {
	caps, e := c.Access(ctx, a, p, id)
	if e != nil {
		return out, e
	}
	if e = domain.SiteReceiptsAccessGuard(a, caps, false); e != nil {
		return out, e
	}
	out.Source, e = c.Source(ctx, p, id)
	if e != nil {
		return out, e
	}
	if out.Source.ProjectID != p || out.Source.PackageID != id {
		return out, domain.SiteReceiptsInvalid("source", "Receipt source does not belong to this project/package")
	}
	out.Record, e = r.Get(ctx, p, id)
	if e != nil {
		return out, e
	}
	out.Actions = []string{}
	out.History = []domain.SiteReceiptsEvent{}
	out.TaskOnly = !caps.Scoped
	if out.Record != nil {
		v := domain.SiteReceiptsCopy(*out.Record)
		out.Record = &v
		if !out.Record.Active {
			return out, domain.SiteReceiptsMissing
		}
		if caps.Scoped {
			out.History, e = r.History(ctx, v.ID)
			if e != nil {
				return out, e
			}
		}
	}
	for _, action := range []string{"record", "revise"} {
		if domain.SiteReceiptsCan(out.Record, a, caps, action, out.Source) == nil {
			out.Actions = append(out.Actions, action)
		}
	}
	return out, nil
}
func (s *SiteReceiptsService) Workspace(ctx context.Context, token, p, id string) (out domain.SiteReceiptsWorkspace, e error) {
	a, e := s.actor(ctx, token, p, id)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.SiteReceiptsRepository, c ports.SiteReceiptsContext) error {
		out, e = s.workspace(ctx, r, c, a, p, id)
		return e
	})
	if e != nil {
		out = domain.SiteReceiptsWorkspace{}
	}
	return
}
func (s *SiteReceiptsService) Execute(ctx context.Context, token, p, id string, q domain.SiteReceiptsCommand) (out domain.SiteReceiptsWorkspace, e error) {
	a, e := s.actor(ctx, token, p, id)
	if e != nil {
		return out, e
	}
	q, e = domain.SiteReceiptsValidate(q)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.SiteReceiptsRepository, c ports.SiteReceiptsContext) error {
		caps, err := c.Access(ctx, a, p, id)
		if err != nil {
			return err
		}
		if err = domain.SiteReceiptsAccessGuard(a, caps, true); err != nil {
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
			return domain.SiteReceiptsStale
		}
		source, err := c.Source(ctx, p, id)
		if err != nil {
			return err
		}
		if source.ProjectID != p || source.PackageID != id {
			return domain.SiteReceiptsInvalid("source", "Receipt source does not belong to this project/package")
		}
		if err = source.Guard(); err != nil {
			return err
		}
		now := s.v.Now()

		next, err := domain.SiteReceiptsApply(before, a, caps, q, source, now, s.v.ID())
		if err != nil {
			return err
		}
		saved, err := r.Save(ctx, next)
		if err != nil {
			return err
		}
		if err = r.Event(ctx, domain.SiteReceiptsEvent{ID: s.v.ID(), ReceiptID: saved.ID, Action: q.Action, Reason: q.Reason, Before: before, After: saved, At: saved.UpdatedAt, ActorID: a.ID}); err != nil {
			return err
		}
		out, err = s.workspace(ctx, r, c, a, p, id)
		return err
	})
	if e != nil {
		out = domain.SiteReceiptsWorkspace{}
	}
	return
}
