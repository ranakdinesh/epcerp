package application

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type WorkspaceService struct {
	u ports.WorkspaceUnitOfWork
	a ports.WorkspaceAuthorizer
	v ports.WorkspaceValues
}

func NewWorkspace(u ports.WorkspaceUnitOfWork, a ports.WorkspaceAuthorizer, v ports.WorkspaceValues) *WorkspaceService {
	return &WorkspaceService{u, a, v}
}
func (s *WorkspaceService) actor(ctx context.Context, token string) (domain.Principal, error) {
	p, e := s.a.Resolve(ctx, token)
	if e != nil {
		return p, e
	}
	if !p.Active || p.ID == "" {
		return p, domain.WorkspaceUnauthorized
	}
	if !p.Can("projects.purchase_workflow.view") || !p.Can("projects.purchase_workflow.read") {
		return p, domain.WorkspaceForbidden
	}
	return p, nil
}
func workspaceScoped(ctx context.Context, p domain.Principal, id string, r ports.WorkspaceRepository, projects ports.WorkspaceProjects, lock bool) (domain.WorkspaceRow, error) {
	project, e := projects.Get(ctx, id, lock)
	if e != nil {
		return domain.WorkspaceRow{}, e
	}
	if project == nil {
		return domain.WorkspaceRow{}, domain.WorkspaceMissing
	}
	w, e := r.Get(ctx, id)
	if e != nil {
		return domain.WorkspaceRow{}, e
	}
	if !domain.WorkspaceCanRead(p, *project, w) {
		return domain.WorkspaceRow{}, domain.WorkspaceForbidden
	}
	return domain.WorkspaceRow{Project: *project, Workspace: w, CanAssign: domain.WorkspaceCanAssign(p, *project) && domain.WorkspaceAssignmentAllowed(*project)}, nil
}
func (s *WorkspaceService) Dashboard(ctx context.Context, token string, f domain.WorkspaceFilter) (domain.WorkspacePage, error) {
	p, e := s.actor(ctx, token)
	if e != nil {
		return domain.WorkspacePage{}, e
	}
	if f, e = f.Validate(); e != nil {
		return domain.WorkspacePage{}, e
	}
	out := domain.WorkspacePage{Rows: []domain.WorkspaceRow{}, Page: f.Page, Size: f.Size, Portfolio: domain.WorkspacePortfolio(p)}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.WorkspaceRepository, projects ports.WorkspaceProjects, _ ports.WorkspaceUsers) error {
		ids, e := r.Assigned(ctx, p.ID)
		if e != nil {
			return e
		}
		rows, total, e := projects.Page(ctx, f, p.ID, ids, domain.WorkspacePortfolio(p))
		if e != nil {
			return e
		}
		out.Total = total
		for _, project := range rows {
			w, e := r.Get(ctx, project.ID)
			if e != nil {
				return e
			}
			if !domain.WorkspaceCanRead(p, project, w) {
				return domain.WorkspaceForbidden
			}
			out.Rows = append(out.Rows, domain.WorkspaceRow{Project: project, Workspace: w, CanAssign: domain.WorkspaceCanAssign(p, project) && domain.WorkspaceAssignmentAllowed(project)})
		}
		indices := domain.WorkspaceSummaryOrder(out.Rows)
		for _, i := range indices {
			row, e := workspaceScoped(ctx, p, out.Rows[i].Project.ID, r, projects, true)
			if e != nil {
				return e
			}
			states, e := r.RegisterStates(ctx, row.Project.ID)
			if e != nil {
				return e
			}
			row.Progress = domain.RegisterCompletion(states)
			out.Rows[i] = row
		}
		return nil
	})
	return out, e
}
func (s *WorkspaceService) Context(ctx context.Context, token, id string) (domain.WorkspaceContext, error) {
	p, e := s.actor(ctx, token)
	if e != nil {
		return domain.WorkspaceContext{}, e
	}
	if id, e = domain.WorkspaceID(id); e != nil {
		return domain.WorkspaceContext{}, e
	}
	out := domain.WorkspaceContext{History: []domain.WorkspaceHistory{}}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.WorkspaceRepository, projects ports.WorkspaceProjects, _ ports.WorkspaceUsers) error {
		var e error
		out.Row, e = workspaceScoped(ctx, p, id, r, projects, true)
		if e != nil {
			return e
		}
		states, e := r.RegisterStates(ctx, id)
		if e != nil {
			return e
		}
		out.Row.Progress = domain.RegisterCompletion(states)
		out.History, e = r.History(ctx, id)
		return e
	})
	return out, e
}
func (s *WorkspaceService) Options(ctx context.Context, token, id string) ([]domain.WorkspaceCandidate, error) {
	p, e := s.actor(ctx, token)
	if e != nil {
		return nil, e
	}
	if id, e = domain.WorkspaceID(id); e != nil {
		return nil, e
	}
	out := []domain.WorkspaceCandidate{}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.WorkspaceRepository, projects ports.WorkspaceProjects, u ports.WorkspaceUsers) error {
		row, e := workspaceScoped(ctx, p, id, r, projects, false)
		if e != nil {
			return e
		}
		if !row.CanAssign {
			return domain.WorkspaceForbidden
		}
		users, e := u.List(ctx)
		if e != nil {
			return e
		}
		for _, c := range users {
			if c.Eligible() {
				out = append(out, c)
			}
		}
		return nil
	})
	return out, e
}
func (s *WorkspaceService) Assign(ctx context.Context, token, id string, input domain.WorkspaceChange) (domain.Workspace, error) {
	p, e := s.actor(ctx, token)
	if e != nil {
		return domain.Workspace{}, e
	}
	if id, e = domain.WorkspaceID(id); e != nil {
		return domain.Workspace{}, e
	}
	if input, e = input.Validate(); e != nil {
		return domain.Workspace{}, e
	}
	var saved domain.Workspace
	e = s.u.Within(ctx, func(ctx context.Context, r ports.WorkspaceRepository, projects ports.WorkspaceProjects, u ports.WorkspaceUsers) error {
		row, e := workspaceScoped(ctx, p, id, r, projects, true)
		if e != nil {
			return e
		}
		if !row.CanAssign {
			return domain.WorkspaceForbidden
		}
		if row.Workspace.Version != input.Version {
			return domain.WorkspaceStale
		}
		if row.Workspace.ManagerID == input.ManagerID {
			return &domain.WorkspaceInvalid{Field: "purchase_manager_id", Message: "Select a different purchase manager"}
		}
		candidate, e := u.Get(ctx, input.ManagerID)
		if e != nil {
			return e
		}
		if candidate == nil || !candidate.Eligible() {
			return &domain.WorkspaceInvalid{Field: "purchase_manager_id", Message: "Select an active user with purchase view and read permissions"}
		}
		saved, e = r.Save(ctx, id, input, p.ID)
		if e != nil {
			return e
		}
		return r.Record(ctx, domain.WorkspaceHistory{ID: s.v.ID(), ProjectID: id, PreviousID: row.Workspace.ManagerID, ManagerID: saved.ManagerID, ActorID: p.ID, Reason: input.Reason, Version: saved.Version, CreatedAt: saved.UpdatedAt})
	})
	return saved, e
}
