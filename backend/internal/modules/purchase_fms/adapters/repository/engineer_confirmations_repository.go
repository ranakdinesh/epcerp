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

type EngineerConfirmationsContextFactory func(database.Connection) ports.EngineerConfirmationsContext
type EngineerConfirmationsUnitOfWork = unitOfWork[ports.EngineerConfirmationsRepository, ports.EngineerConfirmationsContext]

func NewEngineerConfirmationsUnitOfWork(b database.Beginner, c EngineerConfirmationsContextFactory, j func(database.Connection) events.Writer) *EngineerConfirmationsUnitOfWork {
	return newUnitOfWork(b, func(connection database.Connection) (ports.EngineerConfirmationsRepository, ports.EngineerConfirmationsContext) {
		return &engineerConfirmationsRepository{NewStore(connection), j(connection)}, c(connection)
	})
}

type engineerConfirmationsRepository struct {
	q *sqlc.Queries
	j events.Writer
}

var _ ports.EngineerConfirmationsRepository = (*engineerConfirmationsRepository)(nil)

func (r *engineerConfirmationsRepository) Get(ctx context.Context, p, id string) (*domain.EngineerConfirmationsRecord, error) {
	v, e := r.q.EngineerConfirmationsGetConfirmation(ctx, sqlc.EngineerConfirmationsGetConfirmationParams{ProjectID: dtos.UUID(p), PackageID: dtos.UUID(id)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	out, e := dtos.EngineerConfirmationsRecord(v)
	return &out, e
}
func (r *engineerConfirmationsRepository) Save(ctx context.Context, v domain.EngineerConfirmationsRecord) (domain.EngineerConfirmationsRecord, error) {
	data, e := json.Marshal(v)
	if e != nil {
		return domain.EngineerConfirmationsRecord{}, e
	}
	source, e := json.Marshal(v.Source)
	if e != nil {
		return domain.EngineerConfirmationsRecord{}, e
	}
	var row sqlc.PurchaseEngineerConfirmation
	if v.Version == 1 {
		row, e = r.q.EngineerConfirmationsInsertConfirmation(ctx, sqlc.EngineerConfirmationsInsertConfirmationParams{ID: dtos.UUID(v.ID), ProjectID: dtos.UUID(v.ProjectID), PackageID: dtos.UUID(v.PackageID), GrnID: dtos.UUID(v.Source.GRNID), GrnVersion: v.Source.GRNVersion, Revision: v.Revision, Status: v.Confirmation.Status, SourceSnapshot: source, Data: data, At: dtos.Stamp(v.CreatedAt), ActorID: dtos.UUID(v.CreatedBy)})
	} else {
		row, e = r.q.EngineerConfirmationsUpdateConfirmation(ctx, sqlc.EngineerConfirmationsUpdateConfirmationParams{ID: dtos.UUID(v.ID), GrnID: dtos.UUID(v.Source.GRNID), GrnVersion: v.Source.GRNVersion, Revision: v.Revision, Status: v.Confirmation.Status, SourceSnapshot: source, Data: data, At: dtos.Stamp(v.UpdatedAt), ActorID: dtos.UUID(v.UpdatedBy), ExpectedVersion: v.Version - 1})
	}
	var pe *pgconn.PgError
	if errors.Is(e, pgx.ErrNoRows) || errors.As(e, &pe) && pe.Code == "23505" {
		return domain.EngineerConfirmationsRecord{}, domain.EngineerConfirmationsStale
	}
	if e != nil {
		return domain.EngineerConfirmationsRecord{}, e
	}
	return dtos.EngineerConfirmationsRecord(row)
}
func (r *engineerConfirmationsRepository) Event(ctx context.Context, e domain.EngineerConfirmationsEvent) error {
	before, err := json.Marshal(e.Before)
	if err != nil {
		return err
	}
	after, err := json.Marshal(e.After)
	if err != nil {
		return err
	}
	if err = r.q.EngineerConfirmationsEvent(ctx, sqlc.EngineerConfirmationsEventParams{ID: dtos.UUID(e.ID), ConfirmationID: dtos.UUID(e.ConfirmationID), Action: e.Action, Reason: e.Reason, BeforeSnapshot: before, AfterSnapshot: after, At: dtos.Stamp(e.At), ActorID: dtos.UUID(e.ActorID)}); err != nil {
		return err
	}
	return r.j.Record(ctx, events.Event{ID: e.ID, Module: "purchase_engineer_confirmations", EntityType: "purchase_engineer_confirmations", EntityID: e.ConfirmationID, EntityVersion: int64(e.After.Version), Action: e.Action, ActorID: e.ActorID, ActorKind: "user", CorrelationID: e.ID, ScopeKind: "project", ScopeID: e.After.ProjectID, Visibility: "restricted", OccurredAt: e.At}, nil)
}
func (r *engineerConfirmationsRepository) History(ctx context.Context, id string) ([]domain.EngineerConfirmationsEvent, error) {
	rows, e := r.q.EngineerConfirmationsHistory(ctx, dtos.UUID(id))
	if e != nil {
		return nil, e
	}
	out := []domain.EngineerConfirmationsEvent{}
	for _, v := range rows {
		row := domain.EngineerConfirmationsEvent{ID: v.ID.String(), ConfirmationID: v.SubjectID.String(), Action: v.Action, Reason: v.Reason, At: v.CreatedAt.Time.UTC(), ActorID: v.CreatedByID.String()}
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
