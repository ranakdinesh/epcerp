package application

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type ProgressService struct {
	u ports.ProgressUnitOfWork
	a ports.ProgressAuthorizer
	v ports.ProgressValues
}

func NewProgress(u ports.ProgressUnitOfWork, a ports.ProgressAuthorizer, v ports.ProgressValues) *ProgressService {
	return &ProgressService{u, a, v}
}
func (s *ProgressService) Report(ctx context.Context, token, project string) (out domain.ProgressReport, e error) {
	a, e := s.a.Resolve(ctx, token)
	if e != nil || !a.Active || a.ID == "" {
		return out, domain.ProgressUnauthorized
	}
	if !a.Can("projects.purchase_progress.view") || !a.Can("projects.purchase_progress.read") {
		return out, domain.ProgressForbidden
	}
	if e = domain.ProgressID(project); e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.ProgressReader) error {
		if e := r.Authorize(ctx, a, project); e != nil {
			return e
		}
		source, e := r.Source(ctx, project)
		if e != nil {
			return e
		}
		if source.ProjectID != project {
			return domain.ProgressInvalid("project", "Progress source does not match the authorized project")
		}
		if e := r.Authorize(ctx, a, project); e != nil {
			return e
		}
		out, e = domain.ProgressBuild(source, s.v.Now())
		return e
	})
	if e != nil {
		out = domain.ProgressReport{}
	}
	return out, e
}
