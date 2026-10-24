package application

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type TechnicalService struct {
	u ports.TechnicalUnitOfWork
	a ports.TechnicalAuthorizer
	v ports.TechnicalValues
}

func NewTechnical(u ports.TechnicalUnitOfWork, a ports.TechnicalAuthorizer, v ports.TechnicalValues) *TechnicalService {
	return &TechnicalService{u, a, v}
}
func (s *TechnicalService) actor(ctx context.Context, token, p, id string) (domain.Principal, error) {
	a, e := s.a.Resolve(ctx, token)
	if e != nil || !a.Active {
		return a, domain.TechnicalUnauthorized
	}
	for _, v := range []string{a.ID, p, id} {
		if e := domain.TechnicalID(v); e != nil {
			return a, e
		}
	}
	return a, nil
}
func (s *TechnicalService) workspace(ctx context.Context, r ports.TechnicalRepository, c ports.TechnicalContext, a domain.Principal, p, id string) (out domain.TechnicalWorkspace, e error) {
	caps, e := c.Access(ctx, a, p, id)
	if e != nil {
		return out, e
	}
	if !caps.Read {
		return out, domain.TechnicalForbidden
	}
	out.Source, e = c.Source(ctx, p, id)
	if e != nil {
		return out, e
	}
	if out.Source.ProjectID != p || out.Source.PackageID != id {
		return out, domain.TechnicalInvalid("source", "Technical source does not belong to this project/package")
	}
	record, e := r.Get(ctx, p, id)
	if e != nil {
		return out, e
	}
	out.TaskOnly = !caps.Scoped
	out.Files = []domain.TechnicalFile{}
	out.History = []domain.TechnicalEvent{}
	out.Actions = []string{}
	if record != nil {
		if !record.Active {
			return out, domain.TechnicalMissing
		}
		v := domain.TechnicalCopy(*record)
		out.Case = &v
		out.Files, e = r.Files(ctx, v.ID)
		if e != nil {
			return out, e
		}
		if !out.TaskOnly {
			out.History, e = r.History(ctx, v.ID)
			if e != nil {
				return out, e
			}
		}
		out.Ready = domain.TechnicalCurrentReady(v, out.Source, out.Files)
	}
	if out.Source.Guard() != nil || record != nil && domain.TechnicalCurrentSourceGuard(*record, out.Source) != nil {
		return out, nil
	}
	for _, action := range []string{"configure", "document", "submit", "team_review", "client_approval", "revise"} {
		if domain.TechnicalAccessGuard(a, caps, action) != nil {
			continue
		}
		if record == nil {
			if action == "configure" {
				out.Actions = append(out.Actions, action)
			}
			continue
		}
		state := record.State
		allowed := action == "document" && (state == "draft" || state == "revision_required") || action == "submit" && state == "draft" || action == "team_review" && state == "submitted" && record.Submission != nil && record.Submission.SubmittedBy != a.ID || action == "client_approval" && (state == "under_review" || state == "approved" || state == "approved_with_comments") || action == "revise" && state != "draft"
		if allowed {
			out.Actions = append(out.Actions, action)
		}
	}
	return out, nil
}
func (s *TechnicalService) Workspace(ctx context.Context, token, p, id string) (out domain.TechnicalWorkspace, e error) {
	a, e := s.actor(ctx, token, p, id)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.TechnicalRepository, c ports.TechnicalContext) error {
		out, e = s.workspace(ctx, r, c, a, p, id)
		return e
	})
	if e != nil {
		out = domain.TechnicalWorkspace{}
	}
	return
}
func (s *TechnicalService) Execute(ctx context.Context, token, p, id string, q domain.TechnicalCommand) (out domain.TechnicalWorkspace, e error) {
	a, e := s.actor(ctx, token, p, id)
	if e != nil {
		return out, e
	}
	q, e = domain.TechnicalValidateCommand(q)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.TechnicalRepository, c ports.TechnicalContext) error {
		caps, err := c.Access(ctx, a, p, id)
		if err != nil {
			return err
		}
		if err = domain.TechnicalAccessGuard(a, caps, q.Action); err != nil {
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
			return domain.TechnicalStale
		}
		source, err := c.Source(ctx, p, id)
		if err != nil {
			return err
		}
		if source.ProjectID != p || source.PackageID != id {
			return domain.TechnicalInvalid("source", "Technical source does not belong to this project/package")
		}
		files := []domain.TechnicalFile{}
		if before != nil {
			files, err = r.Files(ctx, before.ID)
			if err != nil {
				return err
			}
		}
		ids := []string{}
		if before == nil {
			for range 6 {
				ids = append(ids, s.v.ID())
			}
		}
		next, err := domain.TechnicalApply(before, a, caps, q, source, files, s.v.Now(), s.v.ID(), ids)
		if err != nil {
			return err
		}
		saved, err := r.Save(ctx, next)
		if err != nil {
			return err
		}
		if err = r.Event(ctx, domain.TechnicalEvent{ID: s.v.ID(), CaseID: saved.ID, Action: q.Action, Reason: q.Reason, Before: before, After: saved, At: saved.UpdatedAt, ActorID: a.ID}); err != nil {
			return err
		}
		out, err = s.workspace(ctx, r, c, a, p, id)
		return err
	})
	if e != nil {
		out = domain.TechnicalWorkspace{}
	}
	return
}
