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

type WorkflowFactory func(database.Connection) WorkflowProviders
type WorkflowUnitOfWork = unitOfWork[ports.WorkflowRepository, ports.WorkflowContext]

func NewWorkflowUnitOfWork(b database.Beginner, p WorkflowFactory, j func(database.Connection) events.Writer) *WorkflowUnitOfWork {
	return newUnitOfWork(b, func(connection database.Connection) (ports.WorkflowRepository, ports.WorkflowContext) {
		return &workflowRepository{
			NewStore(connection), j(connection)}, &workflowSourceContext{p(connection)}
	})
}

type workflowRepository struct {
	q *sqlc.Queries
	j events.Writer
}

func (r *workflowRepository) Exists(ctx context.Context, p, indent, demand string) (bool, error) {
	return r.q.WorkflowPackageExists(ctx, sqlc.WorkflowPackageExistsParams{ProjectID: dtos.UUID(p), IndentID: dtos.UUID(indent), DemandID: dtos.UUID(demand)})
}
func (r *workflowRepository) Get(ctx context.Context, p, id string) (*domain.WorkflowPackage, error) {
	row, e := r.q.WorkflowGetPackage(ctx, sqlc.WorkflowGetPackageParams{ProjectID: dtos.UUID(p), ID: dtos.UUID(id)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	v, e := dtos.WorkflowRecord(row)
	return &v, e
}
func (r *workflowRepository) Save(ctx context.Context, v domain.WorkflowPackage, a string) (domain.WorkflowPackage, error) {
	steps, e := json.Marshal(v.Steps)
	if e != nil {
		return v, e
	}
	var row sqlc.PurchaseWorkPackage
	if v.Version == 1 {
		source, e := json.Marshal(v.Source)
		if e != nil {
			return v, e
		}
		row, e = r.q.WorkflowInsertPackage(ctx, sqlc.WorkflowInsertPackageParams{ID: dtos.UUID(v.ID), ProjectID: dtos.UUID(v.ProjectID), IndentID: dtos.UUID(v.IndentID), DemandID: dtos.UUID(v.DemandID), Code: v.Code, Description: v.Description, SourceSnapshot: source, Steps: steps, ActorID: dtos.UUID(a)})
		var pe *pgconn.PgError
		if errors.As(e, &pe) && pe.Code == "23505" {
			return domain.WorkflowPackage{}, domain.WorkflowDuplicate
		}
	} else {
		row, e = r.q.WorkflowUpdatePackage(ctx, sqlc.WorkflowUpdatePackageParams{ProjectID: dtos.UUID(v.ProjectID), ID: dtos.UUID(v.ID), ExpectedVersion: v.Version - 1, Steps: steps, ActorID: dtos.UUID(a)})
	}
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.WorkflowPackage{}, domain.WorkflowStale
	}
	if e != nil {
		return domain.WorkflowPackage{}, e
	}
	return dtos.WorkflowRecord(row)
}
func (r *workflowRepository) Event(ctx context.Context, e domain.WorkflowEvent) error {
	before, err := json.Marshal(e.Before)
	if err != nil {
		return err
	}
	after, err := json.Marshal(e.After)
	if err != nil {
		return err
	}
	if err = r.q.WorkflowEvent(ctx, sqlc.WorkflowEventParams{ID: dtos.UUID(e.ID), SubjectID: dtos.UUID(e.After.ID), Action: e.Action, Reason: e.Reason, BeforeSnapshot: before, AfterSnapshot: after, CreatedAt: pgtype.Timestamptz{Time: e.At, Valid: true}, ActorID: dtos.UUID(e.ActorID)}); err != nil {
		return err
	}
	return r.j.Record(ctx, events.Event{ID: e.ID, Module: "purchase_workflow", EntityType: "purchase_package", EntityID: e.After.ID, EntityVersion: int64(e.After.Version), Action: e.Action, ActorID: e.ActorID, ActorKind: "user", CorrelationID: e.ID, ScopeKind: "project", ScopeID: e.After.ProjectID, Visibility: "restricted", OccurredAt: e.At}, nil)
}
func (r *workflowRepository) History(ctx context.Context, p, id string) ([]domain.WorkflowEvent, error) {
	rows, e := r.q.WorkflowHistory(ctx, sqlc.WorkflowHistoryParams{ProjectID: dtos.UUID(p), ID: dtos.UUID(id)})
	if e != nil {
		return nil, e
	}
	out := []domain.WorkflowEvent{}
	for _, row := range rows {
		v := domain.WorkflowEvent{ID: row.ID.String(), Action: row.Action, Reason: row.Reason, At: row.CreatedAt.Time, ActorID: row.CreatedByID.String()}
		if e = json.Unmarshal(row.BeforeSnapshot, &v.Before); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(row.AfterSnapshot, &v.After); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}
func (r *workflowRepository) Page(ctx context.Context, p string, f domain.WorkflowPageFilter) ([]domain.WorkflowSummary, int64, error) {
	total, e := r.q.WorkflowCountPackages(ctx, sqlc.WorkflowCountPackagesParams{ProjectID: dtos.UUID(p), Search: f.Pattern()})
	if e != nil {
		return nil, 0, e
	}
	rows, e := r.q.WorkflowPagePackages(ctx, sqlc.WorkflowPagePackagesParams{ProjectID: dtos.UUID(p), Search: f.Pattern(), Sort: f.Sort, Direction: f.Direction, PageSize: f.Size, RecordOffset: (f.Page - 1) * f.Size})
	if e != nil {
		return nil, 0, e
	}
	out := []domain.WorkflowSummary{}
	for _, v := range rows {
		out = append(out, domain.WorkflowSummary{Supplier: v.Supplier, LOIState: v.LoiState, TechnicalState: v.TechnicalState, OrderState: v.OrderState, ClientPOState: v.ClientPoState, ID: v.ID.String(), Code: v.Code, Description: v.Description, Quantity: v.Quantity, UOM: v.Uom, Version: v.Version, UpdatedAt: v.UpdatedAt.Time})
	}
	return out, total, nil
}
