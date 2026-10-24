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

type GRNsContextFactory func(database.Connection) ports.GRNsContext
type GRNsUnitOfWork = unitOfWork[ports.GRNsRepository, ports.GRNsContext]

func NewGRNsUnitOfWork(b database.Beginner, c GRNsContextFactory, j func(database.Connection) events.Writer) *GRNsUnitOfWork {
	return newUnitOfWork(b, func(connection database.Connection) (ports.GRNsRepository, ports.GRNsContext) {
		return &gRNsRepository{NewStore(connection), j(connection)}, c(connection)
	})
}

type gRNsRepository struct {
	q *sqlc.Queries
	j events.Writer
}

var _ ports.GRNsRepository = (*gRNsRepository)(nil)

func (r *gRNsRepository) Get(ctx context.Context, p, id string) (*domain.GRNsRecord, error) {
	v, e := r.q.GRNsGetGRN(ctx, sqlc.GRNsGetGRNParams{ProjectID: dtos.UUID(p), PackageID: dtos.UUID(id)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	out, e := dtos.GRNsRecord(v)
	return &out, e
}
func (r *gRNsRepository) Save(ctx context.Context, v domain.GRNsRecord) (domain.GRNsRecord, error) {
	data, e := json.Marshal(v)
	if e != nil {
		return domain.GRNsRecord{}, e
	}
	source, e := json.Marshal(v.Source)
	if e != nil {
		return domain.GRNsRecord{}, e
	}
	var row sqlc.PurchaseGrn
	if v.Version == 1 {
		row, e = r.q.GRNsInsertGRN(ctx, sqlc.GRNsInsertGRNParams{ID: dtos.UUID(v.ID), ProjectID: dtos.UUID(v.ProjectID), PackageID: dtos.UUID(v.PackageID), ReceiptID: dtos.UUID(v.Source.ReceiptID), ReceiptVersion: v.Source.ReceiptVersion, Revision: v.Revision, Status: v.GRN.Status, SourceSnapshot: source, Data: data, At: dtos.Stamp(v.CreatedAt), ActorID: dtos.UUID(v.CreatedBy)})
	} else {
		row, e = r.q.GRNsUpdateGRN(ctx, sqlc.GRNsUpdateGRNParams{ID: dtos.UUID(v.ID), ReceiptID: dtos.UUID(v.Source.ReceiptID), ReceiptVersion: v.Source.ReceiptVersion, Revision: v.Revision, Status: v.GRN.Status, SourceSnapshot: source, Data: data, At: dtos.Stamp(v.UpdatedAt), ActorID: dtos.UUID(v.UpdatedBy), ExpectedVersion: v.Version - 1})
	}
	var pe *pgconn.PgError
	if errors.Is(e, pgx.ErrNoRows) || errors.As(e, &pe) && pe.Code == "23505" {
		return domain.GRNsRecord{}, domain.GRNsStale
	}
	if e != nil {
		return domain.GRNsRecord{}, e
	}
	return dtos.GRNsRecord(row)
}
func (r *gRNsRepository) Event(ctx context.Context, e domain.GRNsEvent) error {
	before, err := json.Marshal(e.Before)
	if err != nil {
		return err
	}
	after, err := json.Marshal(e.After)
	if err != nil {
		return err
	}
	if err = r.q.GRNsEvent(ctx, sqlc.GRNsEventParams{ID: dtos.UUID(e.ID), GrnID: dtos.UUID(e.GRNID), Action: e.Action, Reason: e.Reason, BeforeSnapshot: before, AfterSnapshot: after, At: dtos.Stamp(e.At), ActorID: dtos.UUID(e.ActorID)}); err != nil {
		return err
	}
	return r.j.Record(ctx, events.Event{ID: e.ID, Module: "purchase_grns", EntityType: "purchase_grns", EntityID: e.GRNID, EntityVersion: int64(e.After.Version), Action: e.Action, ActorID: e.ActorID, ActorKind: "user", CorrelationID: e.ID, ScopeKind: "project", ScopeID: e.After.ProjectID, Visibility: "restricted", OccurredAt: e.At}, nil)
}
func (r *gRNsRepository) History(ctx context.Context, id string) ([]domain.GRNsEvent, error) {
	rows, e := r.q.GRNsHistory(ctx, dtos.UUID(id))
	if e != nil {
		return nil, e
	}
	out := []domain.GRNsEvent{}
	for _, v := range rows {
		row := domain.GRNsEvent{ID: v.ID.String(), GRNID: v.SubjectID.String(), Action: v.Action, Reason: v.Reason, At: v.CreatedAt.Time.UTC(), ActorID: v.CreatedByID.String()}
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
