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

type InspectionsContextFactory func(database.Connection) ports.InspectionsContext
type InspectionsUnitOfWork = unitOfWork[ports.InspectionsRepository, ports.InspectionsContext]

func NewInspectionsUnitOfWork(b database.Beginner, c InspectionsContextFactory, j func(database.Connection) events.Writer) *InspectionsUnitOfWork {
	return newUnitOfWork(b, func(connection database.Connection) (ports.InspectionsRepository, ports.InspectionsContext) {
		return &inspectionsRepository{NewStore(connection), j(connection)}, c(connection)
	})
}

type inspectionsRepository struct {
	q *sqlc.Queries
	j events.Writer
}

var _ ports.InspectionsRepository = (*inspectionsRepository)(nil)

func (r *inspectionsRepository) Get(ctx context.Context, p, id string) (*domain.InspectionsRecord, error) {
	v, e := r.q.InspectionsGetInspection(ctx, sqlc.InspectionsGetInspectionParams{ProjectID: dtos.UUID(p), PackageID: dtos.UUID(id)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	out, e := dtos.InspectionsRecord(v)
	return &out, e
}
func (r *inspectionsRepository) Save(ctx context.Context, v domain.InspectionsRecord) (domain.InspectionsRecord, error) {
	data, e := json.Marshal(v)
	if e != nil {
		return domain.InspectionsRecord{}, e
	}
	source, e := json.Marshal(v.Source)
	if e != nil {
		return domain.InspectionsRecord{}, e
	}
	var row sqlc.PurchaseInspection
	if v.Version == 1 {
		row, e = r.q.InspectionsInsertInspection(ctx, sqlc.InspectionsInsertInspectionParams{ID: dtos.UUID(v.ID), ProjectID: dtos.UUID(v.ProjectID), PackageID: dtos.UUID(v.PackageID), OrderID: dtos.UUID(v.Source.OrderID), OrderVersion: v.Source.OrderVersion, AcceptanceID: dtos.UUID(v.Source.AcceptanceID), AcceptanceVersion: v.Source.AcceptanceVersion, TechnicalID: dtos.UUID(v.Source.TechnicalID), TechnicalVersion: v.Source.TechnicalVersion, Revision: v.Revision, Status: v.Inspection.Status, SourceSnapshot: source, Data: data, At: dtos.Stamp(v.CreatedAt), ActorID: dtos.UUID(v.CreatedBy)})
	} else {
		row, e = r.q.InspectionsUpdateInspection(ctx, sqlc.InspectionsUpdateInspectionParams{ID: dtos.UUID(v.ID), Status: v.Inspection.Status, SourceSnapshot: source, OrderID: dtos.UUID(v.Source.OrderID), OrderVersion: v.Source.OrderVersion, AcceptanceID: dtos.UUID(v.Source.AcceptanceID), AcceptanceVersion: v.Source.AcceptanceVersion, TechnicalID: dtos.UUID(v.Source.TechnicalID), TechnicalVersion: v.Source.TechnicalVersion, Revision: v.Revision, Data: data, At: dtos.Stamp(v.UpdatedAt), ActorID: dtos.UUID(v.UpdatedBy), ExpectedVersion: v.Version - 1})
	}
	var pe *pgconn.PgError
	if errors.Is(e, pgx.ErrNoRows) || errors.As(e, &pe) && pe.Code == "23505" {
		return domain.InspectionsRecord{}, domain.InspectionsStale
	}
	if e != nil {
		return domain.InspectionsRecord{}, e
	}
	return dtos.InspectionsRecord(row)
}
func (r *inspectionsRepository) Event(ctx context.Context, e domain.InspectionsEvent) error {
	before, err := json.Marshal(e.Before)
	if err != nil {
		return err
	}
	after, err := json.Marshal(e.After)
	if err != nil {
		return err
	}
	if err = r.q.InspectionsEvent(ctx, sqlc.InspectionsEventParams{ID: dtos.UUID(e.ID), InspectionID: dtos.UUID(e.InspectionID), Action: e.Action, Reason: e.Reason, BeforeSnapshot: before, AfterSnapshot: after, At: dtos.Stamp(e.At), ActorID: dtos.UUID(e.ActorID)}); err != nil {
		return err
	}
	return r.j.Record(ctx, events.Event{ID: e.ID, Module: "purchase_inspections", EntityType: "purchase_inspections", EntityID: e.InspectionID, EntityVersion: int64(e.After.Version), Action: e.Action, ActorID: e.ActorID, ActorKind: "user", CorrelationID: e.ID, ScopeKind: "project", ScopeID: e.After.ProjectID, Visibility: "restricted", OccurredAt: e.At}, nil)
}
func (r *inspectionsRepository) History(ctx context.Context, id string) ([]domain.InspectionsEvent, error) {
	rows, e := r.q.InspectionsHistory(ctx, dtos.UUID(id))
	if e != nil {
		return nil, e
	}
	out := []domain.InspectionsEvent{}
	for _, v := range rows {
		row := domain.InspectionsEvent{ID: v.ID.String(), InspectionID: v.SubjectID.String(), Action: v.Action, Reason: v.Reason, At: v.CreatedAt.Time.UTC(), ActorID: v.CreatedByID.String()}
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
