package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	access "github.com/ranakdinesh/epcERP/internal/modules/project_access/contracts"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type ProgressProviders struct {
	Users    auth.AssignmentDirectory
	Purchase contracts.WorkspaceAccessGate
	Access   access.Gate
	Evidence ports.ProgressSourceReader
}
type ProgressFactory func(database.Connection) ProgressProviders
type ProgressUnitOfWork struct {
	m *database.Manager
	p ProgressFactory
}

func NewProgressUnitOfWork(b database.Beginner, p ProgressFactory) *ProgressUnitOfWork {
	return &ProgressUnitOfWork{database.NewManager(b), p}
}
func (u *ProgressUnitOfWork) Within(ctx context.Context, f func(context.Context, ports.ProgressReader) error) error {
	return u.m.Within(ctx, func(ctx context.Context, c database.Connection) error {
		return f(ctx, &progressReader{c: c, p: u.p(c)})
	})
}

type progressReader struct {
	c       database.Connection
	p       ProgressProviders
	project string
}

func (r *progressReader) Authorize(ctx context.Context, a domain.Principal, project string) error {
	r.project = ""
	if !database.IsTransactionConnection(ctx, r.c) {
		return errors.New("Progress requires the consumer transaction")
	}
	if e := domain.ProgressID(project); e != nil {
		return e
	}
	u, e := r.p.Users.Get(ctx, a.ID)
	if e != nil {
		return e
	}
	if u == nil || u.ID != a.ID || !u.Active || !a.Active {
		return domain.ProgressUnauthorized
	}
	for _, code := range []string{"projects.purchase_progress.view", "projects.purchase_progress.read"} {
		if !a.Can(code) || !u.Allows(code) {
			return domain.ProgressForbidden
		}
	}
	principal := contracts.WorkspacePrincipal{ID: a.ID, Active: true, Superuser: a.Superuser && u.Superuser, Permissions: a.Permissions}
	_, e = r.p.Purchase.Check(ctx, principal, project, "projects.purchase_progress.read", false)
	if errors.Is(e, contracts.WorkspaceErrForbidden) {
		_, e = r.p.Access.Check(ctx, access.Actor{ID: a.ID, Active: true, Superuser: principal.Superuser, Permissions: a.Permissions}, project, "projects.purchase_progress.read", false)
	}
	var scoped *access.Error
	if errors.As(e, &scoped) {
		return &domain.ProgressError{Status: scoped.Status, Field: scoped.Field, Message: scoped.Message}
	}
	if errors.Is(e, contracts.WorkspaceErrForbidden) {
		return domain.ProgressForbidden
	}
	if errors.Is(e, contracts.WorkspaceErrUnauthorized) {
		return domain.ProgressUnauthorized
	}
	if errors.Is(e, contracts.WorkspaceErrMissing) {
		return &domain.ProgressError{Status: 404, Field: "project", Message: "Project not found"}
	}
	if e != nil {
		return e
	}
	r.project = project
	return nil
}
func (r *progressReader) Source(ctx context.Context, project string) (domain.ProgressSource, error) {
	if !database.IsTransactionConnection(ctx, r.c) || r.project == "" || r.project != project {
		return domain.ProgressSource{}, domain.ProgressForbidden
	}
	if r.p.Evidence == nil {
		return domain.ProgressSource{}, errors.New("Progress evidence projection is not configured")
	}
	return r.p.Evidence.Read(ctx, project)
}
