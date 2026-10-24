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

type ClientClearanceContextFactory func(database.Connection) ports.ClientClearanceContext
type ClientClearanceUnitOfWork = unitOfWork[ports.ClientClearanceRepository, ports.ClientClearanceContext]

func NewClientClearanceUnitOfWork(b database.Beginner, c ClientClearanceContextFactory, j func(database.Connection) events.Writer) *ClientClearanceUnitOfWork {
	return newUnitOfWork(b, func(connection database.Connection) (ports.ClientClearanceRepository, ports.ClientClearanceContext) {
		return &clientClearanceRepository{NewStore(connection), j(connection)}, c(connection)
	})
}

type clientClearanceRepository struct {
	q *sqlc.Queries
	j events.Writer
}

var _ ports.ClientClearanceRepository = (*clientClearanceRepository)(nil)

func (r *clientClearanceRepository) Get(ctx context.Context, p, id string) (*domain.ClientClearanceRecord, error) {
	v, e := r.q.ClientClearanceGetClearance(ctx, sqlc.ClientClearanceGetClearanceParams{ProjectID: dtos.UUID(p), PackageID: dtos.UUID(id)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	out, e := dtos.ClientClearanceRecord(v)
	return &out, e
}
func (r *clientClearanceRepository) Save(ctx context.Context, v domain.ClientClearanceRecord) (domain.ClientClearanceRecord, error) {
	data, e := json.Marshal(v)
	if e != nil {
		return domain.ClientClearanceRecord{}, e
	}
	source, e := json.Marshal(v.Source)
	if e != nil {
		return domain.ClientClearanceRecord{}, e
	}
	var row sqlc.PurchaseClientClearance
	if v.Version == 1 {
		row, e = r.q.ClientClearanceInsertClearance(ctx, sqlc.ClientClearanceInsertClearanceParams{ID: dtos.UUID(v.ID), ProjectID: dtos.UUID(v.ProjectID), PackageID: dtos.UUID(v.PackageID), QualityID: dtos.UUID(v.Source.QualityID), QualityVersion: v.Source.QualityVersion, Revision: v.Revision, Status: v.Clearance.Status, SourceSnapshot: source, Data: data, At: dtos.Stamp(v.CreatedAt), ActorID: dtos.UUID(v.CreatedBy)})
	} else {
		row, e = r.q.ClientClearanceUpdateClearance(ctx, sqlc.ClientClearanceUpdateClearanceParams{ID: dtos.UUID(v.ID), QualityID: dtos.UUID(v.Source.QualityID), QualityVersion: v.Source.QualityVersion, Revision: v.Revision, Status: v.Clearance.Status, SourceSnapshot: source, Data: data, At: dtos.Stamp(v.UpdatedAt), ActorID: dtos.UUID(v.UpdatedBy), ExpectedVersion: v.Version - 1})
	}
	var pe *pgconn.PgError
	if errors.Is(e, pgx.ErrNoRows) || errors.As(e, &pe) && pe.Code == "23505" {
		return domain.ClientClearanceRecord{}, domain.ClientClearanceStale
	}
	if e != nil {
		return domain.ClientClearanceRecord{}, e
	}
	return dtos.ClientClearanceRecord(row)
}
func (r *clientClearanceRepository) Event(ctx context.Context, e domain.ClientClearanceEvent) error {
	before, err := json.Marshal(e.Before)
	if err != nil {
		return err
	}
	after, err := json.Marshal(e.After)
	if err != nil {
		return err
	}
	if err = r.q.ClientClearanceEvent(ctx, sqlc.ClientClearanceEventParams{ID: dtos.UUID(e.ID), ClearanceID: dtos.UUID(e.ClearanceID), Action: e.Action, Reason: e.Reason, BeforeSnapshot: before, AfterSnapshot: after, At: dtos.Stamp(e.At), ActorID: dtos.UUID(e.ActorID)}); err != nil {
		return err
	}
	return r.j.Record(ctx, events.Event{ID: e.ID, Module: "purchase_client_clearance", EntityType: "purchase_client_clearance", EntityID: e.ClearanceID, EntityVersion: int64(e.After.Version), Action: e.Action, ActorID: e.ActorID, ActorKind: "user", CorrelationID: e.ID, ScopeKind: "project", ScopeID: e.After.ProjectID, Visibility: "restricted", OccurredAt: e.At}, nil)
}
func (r *clientClearanceRepository) History(ctx context.Context, id string) ([]domain.ClientClearanceEvent, error) {
	rows, e := r.q.ClientClearanceHistory(ctx, dtos.UUID(id))
	if e != nil {
		return nil, e
	}
	out := []domain.ClientClearanceEvent{}
	for _, v := range rows {
		row := domain.ClientClearanceEvent{ID: v.ID.String(), ClearanceID: v.SubjectID.String(), Action: v.Action, Reason: v.Reason, At: v.CreatedAt.Time.UTC(), ActorID: v.CreatedByID.String()}
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
