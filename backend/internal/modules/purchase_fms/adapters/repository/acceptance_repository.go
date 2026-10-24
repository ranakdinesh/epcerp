package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	dtos "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type AcceptanceContextFactory func(database.Connection) ports.AcceptanceContext
type AcceptanceUnitOfWork = unitOfWork[ports.AcceptanceRepository, ports.AcceptanceContext]

func NewAcceptanceUnitOfWork(b database.Beginner, c AcceptanceContextFactory, j func(database.Connection) events.Writer) *AcceptanceUnitOfWork {
	return newUnitOfWork(b, func(connection database.Connection) (ports.AcceptanceRepository, ports.AcceptanceContext) {
		return &acceptanceRepository{NewStore(connection), j(connection)}, c(connection)
	})
}

type acceptanceRepository struct {
	q *sqlc.Queries
	j events.Writer
}

var _ ports.AcceptanceRepository = (*acceptanceRepository)(nil)

func (r *acceptanceRepository) Get(ctx context.Context, p, id string) (*domain.AcceptanceRecord, error) {
	v, e := r.q.AcceptanceGetAcceptance(ctx, sqlc.AcceptanceGetAcceptanceParams{ProjectID: dtos.UUID(p), PackageID: dtos.UUID(id)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	out, e := dtos.AcceptanceRecord(v)
	return &out, e
}
func (r *acceptanceRepository) Save(ctx context.Context, v domain.AcceptanceRecord) (domain.AcceptanceRecord, error) {
	data, e := json.Marshal(v)
	if e != nil {
		return domain.AcceptanceRecord{}, e
	}
	source, e := json.Marshal(v.Order)
	if e != nil {
		return domain.AcceptanceRecord{}, e
	}
	var row sqlc.PurchaseAcceptance
	if v.Version == 1 {
		row, e = r.q.AcceptanceInsertAcceptance(ctx, sqlc.AcceptanceInsertAcceptanceParams{ID: dtos.UUID(v.ID), ProjectID: dtos.UUID(v.ProjectID), PackageID: dtos.UUID(v.PackageID), OrderID: dtos.UUID(v.Order.ID), OrderVersion: v.Order.Version, Status: v.Response.Status, OrderSnapshot: source, Data: data, At: dtos.Stamp(v.CreatedAt), ActorID: dtos.UUID(v.CreatedBy)})
	} else {
		row, e = r.q.AcceptanceUpdateAcceptance(ctx, sqlc.AcceptanceUpdateAcceptanceParams{ID: dtos.UUID(v.ID), Status: v.Response.Status, Data: data, At: dtos.Stamp(v.UpdatedAt), ActorID: dtos.UUID(v.UpdatedBy), ExpectedVersion: v.Version - 1})
	}
	var pe *pgconn.PgError
	if errors.Is(e, pgx.ErrNoRows) || errors.As(e, &pe) && pe.Code == "23505" {
		return domain.AcceptanceRecord{}, domain.AcceptanceStale
	}
	if e != nil {
		return domain.AcceptanceRecord{}, e
	}
	return dtos.AcceptanceRecord(row)
}
func (r *acceptanceRepository) Event(ctx context.Context, e domain.AcceptanceEvent) error {
	before, err := json.Marshal(e.Before)
	if err != nil {
		return err
	}
	after, err := json.Marshal(e.After)
	if err != nil {
		return err
	}
	if err = r.q.AcceptanceEvent(ctx, sqlc.AcceptanceEventParams{ID: dtos.UUID(e.ID), AcceptanceID: dtos.UUID(e.AcceptanceID), Action: e.Action, Reason: e.Reason, BeforeSnapshot: before, AfterSnapshot: after, At: dtos.Stamp(e.At), ActorID: dtos.UUID(e.ActorID)}); err != nil {
		return err
	}
	return r.j.Record(ctx, events.Event{ID: e.ID, Module: "purchase_acceptance", EntityType: "purchase_acceptance", EntityID: e.AcceptanceID, EntityVersion: int64(e.After.Version), Action: e.Action, ActorID: e.ActorID, ActorKind: "user", CorrelationID: e.ID, ScopeKind: "project", ScopeID: e.After.ProjectID, Visibility: "restricted", OccurredAt: e.At}, nil)
}
func (r *acceptanceRepository) History(ctx context.Context, id string) ([]domain.AcceptanceEvent, error) {
	rows, e := r.q.AcceptanceHistory(ctx, dtos.UUID(id))
	if e != nil {
		return nil, e
	}
	out := []domain.AcceptanceEvent{}
	for _, v := range rows {
		row := domain.AcceptanceEvent{ID: v.ID.String(), AcceptanceID: v.SubjectID.String(), Action: v.Action, Reason: v.Reason, At: v.CreatedAt.Time.UTC(), ActorID: v.CreatedByID.String()}
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
