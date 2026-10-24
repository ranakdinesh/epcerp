package application

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type QualityService struct {
	u ports.QualityUnitOfWork
	a ports.QualityAuthorizer
	v ports.QualityValues
}

func NewQuality(u ports.QualityUnitOfWork, a ports.QualityAuthorizer, v ports.QualityValues) *QualityService {
	return &QualityService{u, a, v}
}
func (s *QualityService) actor(ctx context.Context, token, p, id string) (domain.Principal, error) {
	a, e := s.a.Resolve(ctx, token)
	if e != nil || !a.Active {
		return a, domain.QualityUnauthorized
	}
	for _, v := range []string{a.ID, p, id} {
		if e = domain.QualityID(v); e != nil {
			return a, e
		}
	}
	return a, nil
}
func (s *QualityService) workspace(ctx context.Context, r ports.QualityRepository, c ports.QualityContext, a domain.Principal, p, id string) (out domain.QualityWorkspace, e error) {
	caps, e := c.Access(ctx, a, p, id)
	if e != nil {
		return out, e
	}
	if e = domain.QualityAccessGuard(a, caps, false); e != nil {
		return out, e
	}
	out.Source, e = c.Source(ctx, p, id)
	if e != nil {
		return out, e
	}
	if out.Source.ProjectID != p || out.Source.PackageID != id {
		return out, domain.QualityInvalid("source", "Report source does not belong to this project/package")
	}
	out.Record, e = r.Get(ctx, p, id)
	if e != nil {
		return out, e
	}
	out.Actions = []string{}
	out.History = []domain.QualityEvent{}
	out.TaskOnly = !caps.Scoped
	if out.Record != nil {
		v := domain.QualityCopy(*out.Record)
		out.Record = &v
		if !out.Record.Active {
			return out, domain.QualityMissing
		}
		if caps.Scoped {
			out.History, e = r.History(ctx, v.ID)
			if e != nil {
				return out, e
			}
		}
	}
	out.Eligibility = domain.QualityReadiness(out.Record, out.Source, domain.QualityToday(s.v.Now()))
	out.CertificateActions = map[string][]string{}
	for _, cert := range domain.QualityDefaults(a, s.v.Now()) {
		out.CertificateActions[cert.Code] = []string{}
		for _, action := range []string{"record", "verify", "revoke"} {
			if domain.QualityCan(out.Record, a, caps, domain.QualityCommand{Action: action, Code: cert.Code}, out.Source, domain.QualityToday(s.v.Now())) == nil {
				out.CertificateActions[cert.Code] = append(out.CertificateActions[cert.Code], action)
			}
		}
	}
	if domain.QualityCan(out.Record, a, caps, domain.QualityCommand{Action: "revise"}, out.Source, domain.QualityToday(s.v.Now())) == nil {
		out.Actions = append(out.Actions, "revise")
	}

	return out, nil
}
func (s *QualityService) Workspace(ctx context.Context, token, p, id string) (out domain.QualityWorkspace, e error) {
	a, e := s.actor(ctx, token, p, id)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.QualityRepository, c ports.QualityContext) error {
		out, e = s.workspace(ctx, r, c, a, p, id)
		return e
	})
	if e != nil {
		out = domain.QualityWorkspace{}
	}
	return
}
func (s *QualityService) Execute(ctx context.Context, token, p, id string, q domain.QualityCommand) (out domain.QualityWorkspace, e error) {
	a, e := s.actor(ctx, token, p, id)
	if e != nil {
		return out, e
	}
	q, e = domain.QualityValidate(q)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.QualityRepository, c ports.QualityContext) error {
		caps, err := c.Access(ctx, a, p, id)
		if err != nil {
			return err
		}
		if err = domain.QualityActionAccess(a, caps, q.Action); err != nil {
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
			return domain.QualityStale
		}
		source, err := c.Source(ctx, p, id)
		if err != nil {
			return err
		}
		if source.ProjectID != p || source.PackageID != id {
			return domain.QualityInvalid("source", "Report source does not belong to this project/package")
		}
		if err = source.Guard(); err != nil {
			return err
		}
		now := s.v.Now()

		next, err := domain.QualityApply(before, a, caps, q, source, now, s.v.ID())
		if err != nil {
			return err
		}
		saved, err := r.Save(ctx, next)
		if err != nil {
			return err
		}
		if err = r.Event(ctx, domain.QualityEvent{ID: s.v.ID(), CaseID: saved.ID, Code: q.Code, Action: q.Action, Reason: q.Reason, Before: before, After: saved, At: saved.UpdatedAt, ActorID: a.ID}); err != nil {
			return err
		}
		out, err = s.workspace(ctx, r, c, a, p, id)
		return err
	})
	if e != nil {
		out = domain.QualityWorkspace{}
	}
	return
}
