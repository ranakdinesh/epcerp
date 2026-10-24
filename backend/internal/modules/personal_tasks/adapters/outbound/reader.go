package outbound

import (
	"context"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	master "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/personal_tasks/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/personal_tasks/ports"
	purchaseContracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	"time"
)

type Values struct{}

func (Values) Now() domain.Timestamp { return time.Now().UTC() }

type AuthBridge struct{ r auth.Resolver }

func NewAuthBridge(r auth.Resolver) *AuthBridge { return &AuthBridge{r} }
func (b *AuthBridge) Resolve(ctx context.Context, t string) (domain.Actor, error) {
	a, e := b.r.Resolve(ctx, t)
	return domain.Actor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: append([]string{}, a.Permissions...)}, e
}

type Providers struct {
	SupplierReviews master.VendorReviewTasks
	Quotes          purchaseContracts.QuotationWorkReader
	Users           auth.AssignmentDirectory
	Purchase        purchaseContracts.WorkflowTaskReader
}
type UnitOfWork struct {
	m *database.Manager
	p func(database.Connection) Providers
}

func NewUnitOfWork(b database.Beginner, p func(database.Connection) Providers) *UnitOfWork {
	return &UnitOfWork{database.NewManager(b), p}
}
func (u *UnitOfWork) Within(ctx context.Context, f func(context.Context, ports.Reader) error) error {
	return u.m.Within(ctx, func(ctx context.Context, c database.Connection) error { return f(ctx, &reader{c, u.p(c)}) })
}

type reader struct {
	c database.Connection
	p Providers
}

func (r *reader) Authorize(ctx context.Context, a domain.Actor) error {
	if !database.IsTransactionConnection(ctx, r.c) {
		return errors.New("Personal task listing requires its shared transaction")
	}
	u, e := r.p.Users.Get(ctx, a.ID)
	if e != nil {
		return e
	}
	if u == nil || !u.Active || u.ID != a.ID {
		return domain.Unauthorized
	}
	for _, code := range []string{"work.tasks.view", "work.tasks.read"} {
		if !a.Can(code) || !u.Allows(code) {
			return domain.Forbidden
		}
	}
	return nil
}
func (r *reader) Assigned(ctx context.Context, a domain.Actor, q domain.Query) (out []domain.Task, total int64, e error) {
	p, e := r.p.Purchase.Assigned(ctx, purchaseContracts.WorkflowActor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, purchaseContracts.WorkflowTaskQuery{Page: q.Page, Size: q.Size, Search: q.Search, Role: q.Role, State: q.State})
	var d *purchaseContracts.WorkflowError
	if errors.As(e, &d) {
		return nil, 0, &domain.Error{Status: d.Status, Field: d.Field, Message: d.Message}
	}
	if e != nil {
		return nil, 0, e
	}
	out = []domain.Task{}
	for _, t := range p.Rows {
		out = append(out, domain.Task{ID: t.ID, ProjectID: t.ProjectID, ProjectName: t.ProjectName, PackageID: t.PackageID, Code: t.Code, Number: t.Number, Title: t.Title, Phase: t.Phase, Relation: t.Relation, State: t.State, PlannedDate: t.PlannedDate, ResolutionDue: t.ResolutionDue, BlockReason: t.BlockReason, Available: t.Available, Blockers: append([]string{}, t.Blockers...), Version: t.Version})
	}
	return out, p.Total, nil
}

func (r *reader) Quotations(ctx context.Context, a domain.Actor) ([]domain.QuotationTask, error) {
	out := []domain.QuotationTask{}
	if r.p.Quotes == nil {
		return out, nil
	}
	rows, e := r.p.Quotes.Own(ctx, purchaseContracts.WorkflowActor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions})
	if e != nil {
		return nil, e
	}
	for _, row := range rows {
		out = append(out, domain.QuotationTask{ID: row.ID, ProjectID: row.ProjectID, ProjectName: row.ProjectName, Title: row.Title, Link: row.Link, State: row.State})
	}
	return out, nil
}

func (r *reader) SupplierReviews(ctx context.Context, a domain.Actor) ([]domain.SupplierReviewTask, error) {
	out := []domain.SupplierReviewTask{}
	if r.p.SupplierReviews == nil {
		return out, nil
	}
	rows, err := r.p.SupplierReviews.Own(ctx, a.ID)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out = append(out, domain.SupplierReviewTask{ID: row.ID, DocumentID: row.DocumentID, Title: row.Title, Kind: row.Kind, Outcome: row.Outcome, Completed: row.Completed, SupplierID: row.SupplierID, Code: row.Code, Name: row.Name, State: row.State, Message: row.Message, Link: row.Link})
	}
	return out, nil
}
