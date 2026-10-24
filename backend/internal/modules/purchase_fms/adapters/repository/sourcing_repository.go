package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	dtos "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

// ContextFactory supplies this capability's published-source adapter on the same connection.
type SourcingContextFactory func(database.Connection) ports.SourcingContext
type SourcingUnitOfWork = unitOfWork[ports.SourcingRepository, ports.SourcingContext]

func NewSourcingUnitOfWork(b database.Beginner, c SourcingContextFactory, j func(database.Connection) events.Writer) *SourcingUnitOfWork {
	return newUnitOfWork(b, func(connection database.Connection) (ports.SourcingRepository, ports.SourcingContext) {
		return &sourcingRepository{
			NewStore(connection), j(connection)}, c(connection)
	})
}

type sourcingRepository struct {
	q *sqlc.Queries
	j events.Writer
}

func (r *sourcingRepository) Get(ctx context.Context, p, id string) (*domain.SourcingCase, error) {
	v, e := r.q.SourcingGetCase(ctx, sqlc.SourcingGetCaseParams{ProjectID: dtos.UUID(p), PackageID: dtos.UUID(id)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	out, e := dtos.SourcingRecord(v)
	return &out, e
}
func (r *sourcingRepository) Save(ctx context.Context, v domain.SourcingCase, actor string) (domain.SourcingCase, error) {
	data, e := json.Marshal(v)
	if e != nil {
		return domain.SourcingCase{}, e
	}
	source, e := json.Marshal(v.Source)
	if e != nil {
		return domain.SourcingCase{}, e
	}
	var row sqlc.PurchaseSourcingCase
	if v.Version == 1 {
		row, e = r.q.SourcingInsertCase(ctx, sqlc.SourcingInsertCaseParams{ID: dtos.UUID(v.ID), ProjectID: dtos.UUID(v.ProjectID), PackageID: dtos.UUID(v.PackageID), ApproverID: dtos.UUID(v.ApproverID), State: v.State, SourceSnapshot: source, Data: data, CreatedAt: dtos.Stamp(v.CreatedAt), ActorID: dtos.UUID(actor)})
	} else {
		row, e = r.q.SourcingUpdateCase(ctx, sqlc.SourcingUpdateCaseParams{ProjectID: dtos.UUID(v.ProjectID), PackageID: dtos.UUID(v.PackageID), ApproverID: dtos.UUID(v.ApproverID), State: v.State, Data: data, UpdatedAt: dtos.Stamp(v.UpdatedAt), ActorID: dtos.UUID(actor), ExpectedVersion: v.Version - 1})
	}
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.SourcingCase{}, domain.SourcingStale
	}
	var pe *pgconn.PgError
	if errors.As(e, &pe) && pe.Code == "23505" {
		return domain.SourcingCase{}, domain.SourcingStale
	}
	if e != nil {
		return domain.SourcingCase{}, e
	}
	return dtos.SourcingRecord(row)
}
func (r *sourcingRepository) Event(ctx context.Context, e domain.SourcingEvent) error {
	before, err := json.Marshal(e.Before)
	if err != nil {
		return err
	}
	after, err := json.Marshal(e.After)
	if err != nil {
		return err
	}
	if err = r.q.SourcingEvent(ctx, sqlc.SourcingEventParams{ID: dtos.UUID(e.ID), SubjectID: dtos.UUID(e.CaseID), Action: e.Action, Reason: e.Reason, BeforeSnapshot: before, AfterSnapshot: after, CreatedAt: dtos.Stamp(e.At), ActorID: dtos.UUID(e.ActorID)}); err != nil {
		return err
	}
	return r.j.Record(ctx, events.Event{ID: e.ID, Module: "purchase_sourcing", EntityType: "sourcing_case", EntityID: e.CaseID, EntityVersion: int64(e.After.Version), Action: e.Action, ActorID: e.ActorID, ActorKind: "user", CorrelationID: e.ID, ScopeKind: "project", ScopeID: e.After.ProjectID, Visibility: "restricted", OccurredAt: e.At}, nil)
}
func (r *sourcingRepository) History(ctx context.Context, p, id string) ([]domain.SourcingEvent, error) {
	rows, e := r.q.SourcingHistory(ctx, sqlc.SourcingHistoryParams{ProjectID: dtos.UUID(p), PackageID: dtos.UUID(id)})
	if e != nil {
		return nil, e
	}
	out := []domain.SourcingEvent{}
	for _, v := range rows {
		row := domain.SourcingEvent{ID: v.ID.String(), CaseID: v.SubjectID.String(), Action: v.Action, Reason: v.Reason, At: v.CreatedAt.Time, ActorID: v.CreatedByID.String()}
		if e = json.Unmarshal(v.BeforeSnapshot, &row.Before); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(v.AfterSnapshot, &row.After); e != nil {
			return nil, e
		}
		out = append(out, row)
	}
	return out, nil
}

func (r *sourcingRepository) LockAllocation(ctx context.Context, p, demand, id, reference string) (domain.SourcingBalances, error) {
	if err := r.q.SourcingLockAllocation(ctx, p+":"+demand); err != nil {
		return domain.SourcingBalances{}, err
	}
	row, err := r.q.SourcingAllocationBalances(ctx, sqlc.SourcingAllocationBalancesParams{ProjectID: dtos.UUID(p), DemandID: dtos.UUID(demand), CaseID: dtos.UUID(id), FinancialReference: reference})
	return domain.SourcingBalances{Quantity: row.Quantity, Amount: row.Amount}, err
}
func (r *sourcingRepository) StoreAllocation(ctx context.Context, c domain.SourcingCase, p *domain.SourcingProof, a domain.SourcingAllocation) error {
	if c.State == "draft" {
		return r.q.SourcingCancelAllocation(ctx, sqlc.SourcingCancelAllocationParams{CaseID: dtos.UUID(c.ID), CaseVersion: c.Version, At: dtos.Stamp(c.UpdatedAt), ActorID: dtos.UUID(c.UpdatedBy)})
	}
	if p == nil {
		return domain.SourcingInvalid("financial_approval", "Current approval evidence is required")
	}
	numeric := func(s string) (pgtype.Numeric, error) { var n pgtype.Numeric; err := n.Scan(s); return n, err }
	quantity, e := numeric(a.Quantity)
	if e != nil {
		return e
	}
	amount, e := numeric(a.Amount)
	if e != nil {
		return e
	}
	approvedQuantity, e := numeric(p.Quantity)
	if e != nil {
		return e
	}
	approvedAmount, e := numeric(p.Total)
	if e != nil {
		return e
	}
	return r.q.SourcingReserveAllocation(ctx, sqlc.SourcingReserveAllocationParams{CaseID: dtos.UUID(c.ID), ProjectID: dtos.UUID(c.ProjectID), DemandID: dtos.UUID(c.Source.DemandID), FinancialReference: p.Reference + ":" + p.Fingerprint, Currency: c.Source.Currency, Quantity: quantity, Amount: amount, ApprovedQuantity: approvedQuantity, ApprovedAmount: approvedAmount, State: c.State, CaseVersion: c.Version, At: dtos.Stamp(c.UpdatedAt), ActorID: dtos.UUID(c.UpdatedBy)})
}
