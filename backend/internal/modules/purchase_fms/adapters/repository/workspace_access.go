package repository

import (
	"context"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	projects "github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type WorkspaceAccessGate struct {
	projects projects.ContextReader
	clients  masterContracts.ClientDirectory
	repo     *workspaceRepository
}

func NewWorkspaceAccessGate(c database.Connection, p projects.ContextReader, clients masterContracts.ClientDirectory) *WorkspaceAccessGate {
	return &WorkspaceAccessGate{p, clients, &workspaceRepository{q: NewStore(c)}}
}
func (g *WorkspaceAccessGate) Check(ctx context.Context, p domain.Principal, id, permission string, write bool) (domain.WorkspaceAccess, error) {
	return g.check(ctx, p, id, permission, write, false)
}
func (g *WorkspaceAccessGate) CheckHold(ctx context.Context, p domain.Principal, id, permission string, write bool) (domain.WorkspaceAccess, error) {
	return g.check(ctx, p, id, permission, write, true)
}
func (g *WorkspaceAccessGate) check(ctx context.Context, p domain.Principal, id, permission string, write, allowOnHold bool) (domain.WorkspaceAccess, error) {
	if !p.Active || p.ID == "" {
		return domain.WorkspaceAccess{}, domain.WorkspaceUnauthorized
	}
	if !p.Can("projects.purchase_workflow.view") || !p.Can("projects.purchase_workflow.read") || !p.Can(permission) {
		return domain.WorkspaceAccess{}, domain.WorkspaceForbidden
	}
	source, err := g.projects.Get(ctx, id, write)
	if err != nil {
		return domain.WorkspaceAccess{}, err
	}
	if source == nil {
		return domain.WorkspaceAccess{}, domain.WorkspaceMissing
	}
	w, err := g.repo.Get(ctx, id)
	if err != nil {
		return domain.WorkspaceAccess{}, err
	}
	project := workspaceProject(*source)
	if !domain.WorkspaceCanRead(p, project, w) {
		return domain.WorkspaceAccess{}, domain.WorkspaceForbidden
	}
	allowed := domain.WorkspaceCanExecute(p, project, w) && domain.WorkspaceAssignmentAllowed(project)
	if allowOnHold {
		allowed = domain.WorkspaceCanManageHold(p, project, w)
	}
	if write {
		if !allowOnHold && !source.PurchaseEnabled {
			return domain.WorkspaceAccess{}, &domain.WorkspaceInvalid{Field: "purchase_enabled", Message: "Purchase FMS is not enabled for this project"}
		}
		if !allowed {
			return domain.WorkspaceAccess{}, domain.WorkspaceForbidden
		}
		client, err := g.clients.Get(ctx, source.ClientID)
		if err != nil {
			return domain.WorkspaceAccess{}, err
		}
		if client == nil || !client.Active {
			return domain.WorkspaceAccess{}, &domain.WorkspaceInvalid{Field: "client_id", Message: "Project client is inactive"}
		}
	}
	return domain.WorkspaceAccess{ProjectID: id, ClientID: source.ClientID, ProjectManagerID: source.ManagerID, ProcurementExecutiveID: w.ManagerID, ProjectName: source.Name, ProjectNumber: source.Number, CanWrite: allowed}, nil
}
