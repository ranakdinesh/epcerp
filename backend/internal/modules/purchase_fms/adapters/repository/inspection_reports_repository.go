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

type InspectionReportsContextFactory func(database.Connection) ports.InspectionReportsContext
type InspectionReportsUnitOfWork = unitOfWork[ports.InspectionReportsRepository, ports.InspectionReportsContext]

func NewInspectionReportsUnitOfWork(b database.Beginner, c InspectionReportsContextFactory, j func(database.Connection) events.Writer) *InspectionReportsUnitOfWork {
	return newUnitOfWork(b, func(connection database.Connection) (ports.InspectionReportsRepository, ports.InspectionReportsContext) {
		return &inspectionReportsRepository{NewStore(connection), j(connection)}, c(connection)
	})
}

type inspectionReportsRepository struct {
	q *sqlc.Queries
	j events.Writer
}

var _ ports.InspectionReportsRepository = (*inspectionReportsRepository)(nil)

func (r *inspectionReportsRepository) Get(ctx context.Context, p, id string) (*domain.InspectionReportsRecord, error) {
	v, e := r.q.InspectionReportsGetReport(ctx, sqlc.InspectionReportsGetReportParams{ProjectID: dtos.UUID(p), PackageID: dtos.UUID(id)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	out, e := dtos.InspectionReportsRecord(v)
	return &out, e
}
func (r *inspectionReportsRepository) Save(ctx context.Context, v domain.InspectionReportsRecord) (domain.InspectionReportsRecord, error) {
	data, e := json.Marshal(v)
	if e != nil {
		return domain.InspectionReportsRecord{}, e
	}
	source, e := json.Marshal(v.Source)
	if e != nil {
		return domain.InspectionReportsRecord{}, e
	}
	var row sqlc.PurchaseInspectionReport
	if v.Version == 1 {
		row, e = r.q.InspectionReportsInsertReport(ctx, sqlc.InspectionReportsInsertReportParams{ID: dtos.UUID(v.ID), ProjectID: dtos.UUID(v.ProjectID), PackageID: dtos.UUID(v.PackageID), InspectionID: dtos.UUID(v.Source.InspectionID), InspectionVersion: v.Source.InspectionVersion, Revision: v.Revision, Status: v.Report.Status, SourceSnapshot: source, Data: data, At: dtos.Stamp(v.CreatedAt), ActorID: dtos.UUID(v.CreatedBy)})
	} else {
		row, e = r.q.InspectionReportsUpdateReport(ctx, sqlc.InspectionReportsUpdateReportParams{ID: dtos.UUID(v.ID), InspectionID: dtos.UUID(v.Source.InspectionID), InspectionVersion: v.Source.InspectionVersion, Revision: v.Revision, Status: v.Report.Status, SourceSnapshot: source, Data: data, At: dtos.Stamp(v.UpdatedAt), ActorID: dtos.UUID(v.UpdatedBy), ExpectedVersion: v.Version - 1})
	}
	var pe *pgconn.PgError
	if errors.Is(e, pgx.ErrNoRows) || errors.As(e, &pe) && pe.Code == "23505" {
		return domain.InspectionReportsRecord{}, domain.InspectionReportsStale
	}
	if e != nil {
		return domain.InspectionReportsRecord{}, e
	}
	return dtos.InspectionReportsRecord(row)
}
func (r *inspectionReportsRepository) Event(ctx context.Context, e domain.InspectionReportsEvent) error {
	before, err := json.Marshal(e.Before)
	if err != nil {
		return err
	}
	after, err := json.Marshal(e.After)
	if err != nil {
		return err
	}
	if err = r.q.InspectionReportsEvent(ctx, sqlc.InspectionReportsEventParams{ID: dtos.UUID(e.ID), ReportID: dtos.UUID(e.ReportID), Action: e.Action, Reason: e.Reason, BeforeSnapshot: before, AfterSnapshot: after, At: dtos.Stamp(e.At), ActorID: dtos.UUID(e.ActorID)}); err != nil {
		return err
	}
	return r.j.Record(ctx, events.Event{ID: e.ID, Module: "purchase_inspection_reports", EntityType: "purchase_inspection_reports", EntityID: e.ReportID, EntityVersion: int64(e.After.Version), Action: e.Action, ActorID: e.ActorID, ActorKind: "user", CorrelationID: e.ID, ScopeKind: "project", ScopeID: e.After.ProjectID, Visibility: "restricted", OccurredAt: e.At}, nil)
}
func (r *inspectionReportsRepository) History(ctx context.Context, id string) ([]domain.InspectionReportsEvent, error) {
	rows, e := r.q.InspectionReportsHistory(ctx, dtos.UUID(id))
	if e != nil {
		return nil, e
	}
	out := []domain.InspectionReportsEvent{}
	for _, v := range rows {
		row := domain.InspectionReportsEvent{ID: v.ID.String(), ReportID: v.SubjectID.String(), Action: v.Action, Reason: v.Reason, At: v.CreatedAt.Time.UTC(), ActorID: v.CreatedByID.String()}
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
