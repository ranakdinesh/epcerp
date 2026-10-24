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

type DispatchContextFactory func(database.Connection) ports.DispatchContext
type DispatchUnitOfWork = unitOfWork[ports.DispatchRepository, ports.DispatchContext]

func NewDispatchUnitOfWork(b database.Beginner, c DispatchContextFactory, j func(database.Connection) events.Writer) *DispatchUnitOfWork {
	return newUnitOfWork(b, func(connection database.Connection) (ports.DispatchRepository, ports.DispatchContext) {
		return &dispatchRepository{
			NewStore(connection), j(connection)}, c(connection)
	})
}

type dispatchRepository struct {
	q *sqlc.Queries
	j events.Writer
}

var _ ports.DispatchRepository = (*dispatchRepository)(nil)

func (r *dispatchRepository) Get(ctx context.Context, p, id string) (*domain.DispatchRecord, error) {
	v, e := r.q.DispatchGetDispatch(ctx, sqlc.DispatchGetDispatchParams{ProjectID: dtos.UUID(p), PackageID: dtos.UUID(id)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	out, e := dtos.DispatchRecord(v)
	return &out, e
}
func (r *dispatchRepository) Save(ctx context.Context, v domain.DispatchRecord) (domain.DispatchRecord, error) {
	data, e := json.Marshal(v)
	if e != nil {
		return domain.DispatchRecord{}, e
	}
	source, e := json.Marshal(v.Source)
	if e != nil {
		return domain.DispatchRecord{}, e
	}
	var row sqlc.PurchaseDispatch
	if v.Version == 1 {
		row, e = r.q.DispatchInsertDispatch(ctx, sqlc.DispatchInsertDispatchParams{ID: dtos.UUID(v.ID), ProjectID: dtos.UUID(v.ProjectID), PackageID: dtos.UUID(v.PackageID), ClearanceID: dtos.UUID(v.Source.ClearanceID), ClearanceVersion: v.Source.ClearanceVersion, Revision: v.Revision, Status: v.Dispatch.Status, SourceSnapshot: source, Data: data, At: dtos.Stamp(v.CreatedAt), ActorID: dtos.UUID(v.CreatedBy)})
	} else {
		row, e = r.q.DispatchUpdateDispatch(ctx, sqlc.DispatchUpdateDispatchParams{ID: dtos.UUID(v.ID), ClearanceID: dtos.UUID(v.Source.ClearanceID), ClearanceVersion: v.Source.ClearanceVersion, Revision: v.Revision, Status: v.Dispatch.Status, SourceSnapshot: source, Data: data, At: dtos.Stamp(v.UpdatedAt), ActorID: dtos.UUID(v.UpdatedBy), ExpectedVersion: v.Version - 1})
	}
	var pe *pgconn.PgError
	if errors.Is(e, pgx.ErrNoRows) || errors.As(e, &pe) && pe.Code == "23505" {
		return domain.DispatchRecord{}, domain.DispatchStale
	}
	if e != nil {
		return domain.DispatchRecord{}, e
	}
	return dtos.DispatchRecord(row)
}
func (r *dispatchRepository) Event(ctx context.Context, e domain.DispatchEvent) error {
	before, err := json.Marshal(e.Before)
	if err != nil {
		return err
	}
	after, err := json.Marshal(e.After)
	if err != nil {
		return err
	}
	if err = r.q.DispatchEvent(ctx, sqlc.DispatchEventParams{ID: dtos.UUID(e.ID), DispatchID: dtos.UUID(e.DispatchID), Action: e.Action, Reason: e.Reason, BeforeSnapshot: before, AfterSnapshot: after, At: dtos.Stamp(e.At), ActorID: dtos.UUID(e.ActorID)}); err != nil {
		return err
	}
	return r.j.Record(ctx, events.Event{ID: e.ID, Module: "purchase_dispatch", EntityType: "purchase_dispatch", EntityID: e.DispatchID, EntityVersion: int64(e.After.Version), Action: e.Action, ActorID: e.ActorID, ActorKind: "user", CorrelationID: e.ID, ScopeKind: "project", ScopeID: e.After.ProjectID, Visibility: "restricted", OccurredAt: e.At}, nil)
}
func (r *dispatchRepository) History(ctx context.Context, id string) ([]domain.DispatchEvent, error) {
	rows, e := r.q.DispatchHistory(ctx, dtos.UUID(id))
	if e != nil {
		return nil, e
	}
	out := []domain.DispatchEvent{}
	for _, v := range rows {
		row := domain.DispatchEvent{ID: v.ID.String(), DispatchID: v.SubjectID.String(), Action: v.Action, Reason: v.Reason, At: v.CreatedAt.Time.UTC(), ActorID: v.CreatedByID.String()}
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
