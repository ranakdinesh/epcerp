package outbound

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/ports"
)

type Factory func(database.Connection) Providers
type UnitOfWork struct {
	m *database.Manager
	p Factory
	j func(database.Connection) events.Writer
	f func(database.Connection) ports.Approvals
}

func NewUnitOfWork(b database.Beginner, p Factory, j func(database.Connection) events.Writer) *UnitOfWork {
	return &UnitOfWork{m: database.NewManager(b), p: p, j: j}
}

// NewFinanceUnitOfWork binds the real approval publication to the same transaction.
func NewFinanceUnitOfWork(b database.Beginner, p Factory, j func(database.Connection) events.Writer, f func(database.Connection) ports.Approvals) *UnitOfWork {
	u := NewUnitOfWork(b, p, j)
	u.f = f
	return u
}
func (u *UnitOfWork) Within(ctx context.Context, f func(context.Context, ports.Repository, ports.Context, ports.Approvals) error) error {
	return u.m.Within(ctx, func(ctx context.Context, c database.Connection) error {
		var finance ports.Approvals
		if u.f != nil {
			finance = u.f(c)
		}
		return f(ctx, &repository{db.New(c), u.j(c)}, &sourceContext{u.p(c)}, finance)
	})
}
func uuid(s string) pgtype.UUID { var n pgtype.UUID; _ = n.Scan(s); return n }
func nullable(s *string) pgtype.UUID {
	if s == nil {
		return pgtype.UUID{}
	}
	return uuid(*s)
}
func pointer(n pgtype.UUID) *string {
	if !n.Valid {
		return nil
	}
	s := n.String()
	return &s
}
func numeric(s *string) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	if s == nil {
		return n, nil
	}
	e := n.Scan(*s)
	return n, e
}
func number(n pgtype.Numeric) (string, error) {
	v, e := n.Value()
	if e != nil || v == nil {
		return "", e
	}
	s, ok := v.(string)
	if !ok {
		return "", errors.New("invalid stored budget decimal")
	}
	return s, nil
}
func numptr(n pgtype.Numeric) (*string, error) {
	if !n.Valid {
		return nil, nil
	}
	s, e := number(n)
	return &s, e
}
func stamp(t domain.Timestamp) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

type repository struct {
	q *db.Queries
	j events.Writer
}

func persistenceError(e error) error {
	var p *pgconn.PgError
	if errors.As(e, &p) {
		if p.Code == "23503" {
			return domain.Invalid("data", "A budget reference is unavailable or belongs to another project")
		}
		if p.Code == "23505" {
			return domain.Invalid("data", "A budget record or active allocation already exists")
		}
	}
	return e
}

func (r *repository) Get(ctx context.Context, p string) (*domain.Budget, error) {
	row, e := r.q.GetBudget(ctx, uuid(p))
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	total, e := number(row.Total)
	if e != nil {
		return nil, e
	}
	v := domain.Budget{ProjectID: p, Version: row.Version, Revision: row.Revision, State: row.State, Active: row.IsActive, Audit: domain.Audit{CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time, CreatedBy: pointer(row.CreatedByID), UpdatedBy: pointer(row.UpdatedByID)}, Data: domain.Data{Total: total, Currency: row.Currency, OwnerID: row.OwnerID.String(), Notes: row.Notes, PricingMode: row.PricingMode, Heads: []domain.Head{}, Allocations: []domain.Allocation{}}}
	if e = json.Unmarshal(row.ApprovedSnapshot, &v.Approved); e != nil {
		return nil, e
	}
	heads, e := r.q.BudgetHeads(ctx, uuid(p))
	if e != nil {
		return nil, e
	}
	for _, h := range heads {
		amount, e := number(h.Amount)
		if e != nil {
			return nil, e
		}
		v.Data.Heads = append(v.Data.Heads, domain.Head{ID: h.ID.String(), ParentID: pointer(h.ParentID), Name: h.Name, Kind: h.Kind, Amount: amount, OwnerID: h.OwnerID.String(), Active: h.IsActive, Audit: domain.Audit{CreatedAt: h.CreatedAt.Time, UpdatedAt: h.UpdatedAt.Time, CreatedBy: pointer(h.CreatedByID), UpdatedBy: pointer(h.UpdatedByID)}})
	}
	allocations, e := r.q.BudgetAllocations(ctx, uuid(p))
	if e != nil {
		return nil, e
	}
	for _, a := range allocations {
		amount, e := number(a.Amount)
		if e != nil {
			return nil, e
		}
		price, e := numptr(a.UnitPrice)
		if e != nil {
			return nil, e
		}
		qty, e := numptr(a.PricingQuantity)
		if e != nil {
			return nil, e
		}
		allocation := domain.Allocation{ID: a.ID.String(), BOID: a.BoID.String(), LineID: a.LineID.String(), HeadID: a.HeadID.String(), Amount: amount, OwnerID: a.OwnerID.String(), Active: a.IsActive, UnitPrice: price, PricingQuantity: qty, Audit: domain.Audit{CreatedAt: a.CreatedAt.Time, UpdatedAt: a.UpdatedAt.Time, CreatedBy: pointer(a.CreatedByID), UpdatedBy: pointer(a.UpdatedByID)}}
		if e = json.Unmarshal(a.SourceSnapshot, &allocation.Source); e != nil {
			return nil, e
		}
		v.Data.Allocations = append(v.Data.Allocations, allocation)
	}
	return &v, nil
}
func (r *repository) Save(ctx context.Context, v domain.Budget, actor string) (domain.Budget, error) {
	approved, e := json.Marshal(v.Approved)
	if e != nil {
		return v, e
	}
	var decision *string
	if v.Approved != nil {
		decision = &v.Approved.DecisionID
	}
	total, e := numeric(&v.Data.Total)
	if e != nil {
		return v, persistenceError(e)
	}
	if v.Version == 1 {
		_, e = r.q.InsertBudget(ctx, db.InsertBudgetParams{ProjectID: uuid(v.ProjectID), Revision: v.Revision, Total: total, Currency: v.Data.Currency, OwnerID: uuid(v.Data.OwnerID), Notes: v.Data.Notes, PricingMode: v.Data.PricingMode, ActorID: uuid(actor)})
	} else {
		_, e = r.q.UpdateBudget(ctx, db.UpdateBudgetParams{ProjectID: uuid(v.ProjectID), ExpectedVersion: v.Version - 1, Revision: v.Revision, State: v.State, ApprovedSnapshot: approved, ApprovalDecisionID: nullable(decision), Total: total, Currency: v.Data.Currency, OwnerID: uuid(v.Data.OwnerID), Notes: v.Data.Notes, PricingMode: v.Data.PricingMode, IsActive: v.Active, ActorID: uuid(actor)})
	}
	if errors.Is(e, pgx.ErrNoRows) {
		return v, domain.Stale
	}
	if e != nil {
		return v, persistenceError(e)
	}
	for n, h := range v.Data.Heads {
		amount, e := numeric(&h.Amount)
		if e != nil {
			return v, persistenceError(e)
		}
		affected, e := r.q.SaveBudgetHead(ctx, db.SaveBudgetHeadParams{ID: uuid(h.ID), ProjectID: uuid(v.ProjectID), SortOrder: int32(n), ParentID: nullable(h.ParentID), Name: h.Name, Kind: h.Kind, Amount: amount, OwnerID: uuid(h.OwnerID), IsActive: h.Active, CreatedAt: stamp(h.CreatedAt), UpdatedAt: stamp(h.UpdatedAt), CreatedByID: nullable(h.CreatedBy), ActorID: uuid(actor)})
		if e != nil {
			return v, persistenceError(e)
		}
		if affected != 1 {
			return v, domain.Invalid("heads", "A head identifier belongs to a different project")
		}
	}
	for _, active := range []bool{false, true} {
		for n, a := range v.Data.Allocations {
			if a.Active != active {
				continue
			}
			amount, e := numeric(&a.Amount)
			if e != nil {
				return v, persistenceError(e)
			}
			price, e := numeric(a.UnitPrice)
			if e != nil {
				return v, persistenceError(e)
			}
			qty, e := numeric(a.PricingQuantity)
			if e != nil {
				return v, persistenceError(e)
			}
			snapshot, e := json.Marshal(a.Source)
			if e != nil {
				return v, persistenceError(e)
			}
			affected, e := r.q.SaveBudgetAllocation(ctx, db.SaveBudgetAllocationParams{ID: uuid(a.ID), ProjectID: uuid(v.ProjectID), SortOrder: int32(n), BoID: uuid(a.BOID), LineID: uuid(a.LineID), HeadID: uuid(a.HeadID), Amount: amount, OwnerID: uuid(a.OwnerID), IsActive: a.Active, UnitPrice: price, PricingQuantity: qty, SourceSnapshot: snapshot, CreatedAt: stamp(a.CreatedAt), UpdatedAt: stamp(a.UpdatedAt), CreatedByID: nullable(a.CreatedBy), ActorID: uuid(actor)})
			if e != nil {
				return v, persistenceError(e)
			}
			if affected != 1 {
				return v, domain.Invalid("allocations", "An allocation identifier belongs to a different project or source item")
			}
		}
	}
	saved, e := r.Get(ctx, v.ProjectID)
	if e != nil {
		return v, persistenceError(e)
	}
	if saved == nil {
		return v, domain.Missing
	}
	return *saved, nil
}
func (r *repository) Event(ctx context.Context, v domain.Event) error {
	before, e := json.Marshal(v.Before)
	if e != nil {
		return e
	}
	after, e := json.Marshal(v.After)
	if e != nil {
		return e
	}
	if e = r.q.InsertBudgetEvent(ctx, db.InsertBudgetEventParams{ID: uuid(v.ID), ProjectID: uuid(v.ProjectID), Action: v.Action, Reason: v.Reason, BeforeSnapshot: before, AfterSnapshot: after, ActorID: uuid(v.ActorID)}); e != nil {
		return e
	}
	return r.j.Record(ctx, events.Event{ID: v.ID, Module: "project_budgets", EntityType: "project_budget", EntityID: v.ProjectID, EntityVersion: int64(v.After.Version), Action: v.Action, ActorID: v.ActorID, ActorKind: "user", CorrelationID: v.ID, ScopeKind: "project", ScopeID: v.ProjectID, Visibility: "restricted", OccurredAt: v.CreatedAt}, nil)
}
func (r *repository) History(ctx context.Context, p string) ([]domain.Event, error) {
	rows, e := r.q.BudgetHistory(ctx, uuid(p))
	if e != nil {
		return nil, e
	}
	out := []domain.Event{}
	for _, row := range rows {
		event := domain.Event{ID: row.ID.String(), ProjectID: row.ProjectID.String(), Action: row.Action, Reason: row.Reason, ActorID: row.CreatedByID.String(), CreatedAt: row.CreatedAt.Time}
		if e = json.Unmarshal(row.BeforeSnapshot, &event.Before); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(row.AfterSnapshot, &event.After); e != nil {
			return nil, e
		}
		out = append(out, event)
	}
	return out, nil
}
