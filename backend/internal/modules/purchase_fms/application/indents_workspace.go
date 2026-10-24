package application

import (
	"context"
	"errors"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

func (s *IndentsService) Workspace(ctx context.Context, token, project string) (out domain.IndentsWorkspace, e error) {
	a, e := s.actor(ctx, token, "projects.purchase_indents.read", "projects.purchase_indents.view")
	if e != nil {
		return out, e
	}
	if project, e = domain.IndentsID(project); e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.IndentsRepository, c ports.IndentsContext) error {
		if e := c.Scope(ctx, a, project, "projects.purchase_indents.read", false); e != nil {
			return e
		}
		rows, e := r.List(ctx, project)
		if e != nil {
			return e
		}
		permissions := map[string]bool{"purchase_indents.read": true}
		for _, action := range []string{"write", "review", "release"} {
			code := "projects.purchase_indents." + action
			if !a.Can(code) {
				continue
			}
			e = c.Scope(ctx, a, project, code, true)
			if e != nil {
				var invalid *domain.IndentsError
				if errors.Is(e, domain.IndentsForbidden) || errors.As(e, &invalid) && invalid.Status == 422 {
					continue
				}
				return e
			}
			permissions["purchase_indents."+action] = true
		}
		source, e := c.WorkspaceSource(ctx, project, permissions["purchase_indents.write"], rows)
		if e != nil {
			return e
		}
		out = domain.IndentsWorkspace{IndentsWorkspaceSource: source, Rows: rows, Permissions: permissions}
		return nil
	})
	if e != nil {
		out = domain.IndentsWorkspace{}
	}
	return out, e
}
