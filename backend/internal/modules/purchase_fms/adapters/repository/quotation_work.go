package repository

import (
	"context"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	access "github.com/ranakdinesh/epcERP/internal/modules/project_access/contracts"
	projects "github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"
	dtos "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/dtos"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type quotationWorkReader struct {
	c        database.Connection
	users    auth.AssignmentDirectory
	purchase contracts.WorkspaceAccessGate
	accounts access.Gate
	projects projects.ContextReader
}

func NewQuotationWorkReader(c database.Connection, u auth.AssignmentDirectory, p contracts.WorkspaceAccessGate, a access.Gate, j projects.ContextReader) contracts.QuotationWorkReader {
	return &quotationWorkReader{c, u, p, a, j}
}
func (r *quotationWorkReader) Own(ctx context.Context, a domain.Principal) ([]domain.QuotationWorkTask, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, errors.New("Quotation work requires consumer transaction")
	}
	u, e := r.users.Get(ctx, a.ID)
	if e != nil {
		return nil, e
	}
	if u == nil || !u.Active {
		return nil, domain.QuotationApprovalsUnauthorized
	}
	a.Superuser = a.Superuser && u.Superuser
	rows, e := NewStore(r.c).QuotationWorkOwn(ctx, dtos.UUID(a.ID))
	if e != nil {
		return nil, e
	}
	out := []domain.QuotationWorkTask{}
	for _, row := range rows {
		permission := "projects.group_comparisons.read"
		if row.Kind == "finance" {
			permission = "accounts.quotation_approvals.read"
		}
		if !a.Can(permission) || !u.Allows(permission) {
			continue
		}
		p := row.ProjectID.String()
		var allowed bool
		if row.Kind == "finance" {
			_, e = r.accounts.Check(ctx, access.Actor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, permission, false)
			allowed = e == nil
		} else {
			_, e = r.purchase.Check(ctx, contracts.WorkspacePrincipal{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, permission, false)
			allowed = e == nil
		}
		ok, err := quotationApprovalsSuppress(e)
		if err != nil {
			return nil, err
		}
		if !allowed || !ok {
			continue
		}
		project, e := r.projects.Get(ctx, p, false)
		if e != nil {
			return nil, e
		}
		if project == nil {
			continue
		}
		out = append(out, domain.QuotationWorkTask{ID: row.ID.String(), ProjectID: p, ProjectName: project.Name, BOID: row.BoID.String(), GroupID: row.GroupID.String(), Kind: row.Kind, State: row.State, Title: row.Title, Link: row.Link})
	}
	return out, nil
}
