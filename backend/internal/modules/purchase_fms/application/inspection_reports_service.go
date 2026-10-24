package application

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type InspectionReportsService struct {
	u ports.InspectionReportsUnitOfWork
	a ports.InspectionReportsAuthorizer
	v ports.InspectionReportsValues
}

func NewInspectionReports(u ports.InspectionReportsUnitOfWork, a ports.InspectionReportsAuthorizer, v ports.InspectionReportsValues) *InspectionReportsService {
	return &InspectionReportsService{u, a, v}
}
func (s *InspectionReportsService) actor(ctx context.Context, token, p, id string) (domain.Principal, error) {
	a, e := s.a.Resolve(ctx, token)
	if e != nil || !a.Active {
		return a, domain.InspectionReportsUnauthorized
	}
	for _, v := range []string{a.ID, p, id} {
		if e = domain.InspectionReportsID(v); e != nil {
			return a, e
		}
	}
	return a, nil
}
func (s *InspectionReportsService) workspace(ctx context.Context, r ports.InspectionReportsRepository, c ports.InspectionReportsContext, a domain.Principal, p, id string) (out domain.InspectionReportsWorkspace, e error) {
	caps, e := c.Access(ctx, a, p, id)
	if e != nil {
		return out, e
	}
	if e = domain.InspectionReportsAccessGuard(a, caps, false); e != nil {
		return out, e
	}
	out.Source, e = c.Source(ctx, p, id)
	if e != nil {
		return out, e
	}
	if out.Source.ProjectID != p || out.Source.PackageID != id {
		return out, domain.InspectionReportsInvalid("source", "Report source does not belong to this project/package")
	}
	out.Record, e = r.Get(ctx, p, id)
	if e != nil {
		return out, e
	}
	out.Actions = []string{}
	out.History = []domain.InspectionReportsEvent{}
	out.TaskOnly = !caps.Scoped
	if out.Record != nil {
		v := domain.InspectionReportsCopy(*out.Record)
		out.Record = &v
		if !out.Record.Active {
			return out, domain.InspectionReportsMissing
		}
		if caps.Scoped {
			out.History, e = r.History(ctx, v.ID)
			if e != nil {
				return out, e
			}
		}
	}
	for _, action := range []string{"record", "revise"} {
		if domain.InspectionReportsCan(out.Record, a, caps, action, out.Source) == nil {
			out.Actions = append(out.Actions, action)
		}
	}
	return out, nil
}
func (s *InspectionReportsService) Workspace(ctx context.Context, token, p, id string) (out domain.InspectionReportsWorkspace, e error) {
	a, e := s.actor(ctx, token, p, id)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.InspectionReportsRepository, c ports.InspectionReportsContext) error {
		out, e = s.workspace(ctx, r, c, a, p, id)
		return e
	})
	if e != nil {
		out = domain.InspectionReportsWorkspace{}
	}
	return
}
func (s *InspectionReportsService) Execute(ctx context.Context, token, p, id string, q domain.InspectionReportsCommand) (out domain.InspectionReportsWorkspace, e error) {
	a, e := s.actor(ctx, token, p, id)
	if e != nil {
		return out, e
	}
	q, e = domain.InspectionReportsValidate(q)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.InspectionReportsRepository, c ports.InspectionReportsContext) error {
		caps, err := c.Access(ctx, a, p, id)
		if err != nil {
			return err
		}
		if err = domain.InspectionReportsAccessGuard(a, caps, true); err != nil {
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
			return domain.InspectionReportsStale
		}
		source, err := c.Source(ctx, p, id)
		if err != nil {
			return err
		}
		if source.ProjectID != p || source.PackageID != id {
			return domain.InspectionReportsInvalid("source", "Report source does not belong to this project/package")
		}
		if err = source.Guard(); err != nil {
			return err
		}
		now := s.v.Now()

		next, err := domain.InspectionReportsApply(before, a, caps, q, source, now, s.v.ID())
		if err != nil {
			return err
		}
		saved, err := r.Save(ctx, next)
		if err != nil {
			return err
		}
		if err = r.Event(ctx, domain.InspectionReportsEvent{ID: s.v.ID(), ReportID: saved.ID, Action: q.Action, Reason: q.Reason, Before: before, After: saved, At: saved.UpdatedAt, ActorID: a.ID}); err != nil {
			return err
		}
		out, err = s.workspace(ctx, r, c, a, p, id)
		return err
	})
	if e != nil {
		out = domain.InspectionReportsWorkspace{}
	}
	return
}
