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

type OrdersContextFactory func(database.Connection) ports.OrdersContext
type OrdersUnitOfWork = unitOfWork[ports.OrdersRepository, ports.OrdersContext]

func NewOrdersUnitOfWork(b database.Beginner, c OrdersContextFactory, j func(database.Connection) events.Writer) *OrdersUnitOfWork {
	return newUnitOfWork(b, func(connection database.Connection) (ports.OrdersRepository, ports.OrdersContext) {
		return &ordersRepository{NewStore(connection), j(connection)}, c(connection)
	})
}

type ordersRepository struct {
	q *sqlc.Queries
	j events.Writer
}

var _ ports.OrdersRepository = (*ordersRepository)(nil)

func (r *ordersRepository) Get(ctx context.Context, p, id string) (*domain.OrdersOrder, error) {
	v, e := r.q.OrdersGetOrder(ctx, sqlc.OrdersGetOrderParams{ProjectID: dtos.UUID(p), PackageID: dtos.UUID(id)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	out, e := dtos.OrdersRecord(v)
	return &out, e
}
func (r *ordersRepository) Save(ctx context.Context, v domain.OrdersOrder) (domain.OrdersOrder, error) {
	data, e := json.Marshal(v)
	if e != nil {
		return domain.OrdersOrder{}, e
	}
	source, e := json.Marshal(v.Source)
	if e != nil {
		return domain.OrdersOrder{}, e
	}
	var row sqlc.PurchaseOrder
	if v.Version == 1 {
		row, e = r.q.OrdersInsertOrder(ctx, sqlc.OrdersInsertOrderParams{ID: dtos.UUID(v.ID), ProjectID: dtos.UUID(v.ProjectID), PackageID: dtos.UUID(v.PackageID), SourcingID: dtos.UUID(v.Source.SourcingID), TechnicalID: dtos.UUID(v.Source.Technical.CaseID), SourceSnapshot: source, Data: data, At: dtos.Stamp(v.CreatedAt), ActorID: dtos.UUID(v.CreatedBy)})
	} else {
		row, e = r.q.OrdersUpdateOrder(ctx, sqlc.OrdersUpdateOrderParams{ID: dtos.UUID(v.ID), State: v.State, Number: pgtype.Text{String: v.Number, Valid: v.Number != ""}, SourceSnapshot: source, Data: data, At: dtos.Stamp(v.UpdatedAt), ActorID: dtos.UUID(v.UpdatedBy), ExpectedVersion: v.Version - 1})
	}
	var pe *pgconn.PgError
	if errors.Is(e, pgx.ErrNoRows) || errors.As(e, &pe) && pe.Code == "23505" {
		return domain.OrdersOrder{}, domain.OrdersStale
	}
	if e != nil {
		return domain.OrdersOrder{}, e
	}
	return dtos.OrdersRecord(row)
}
func (r *ordersRepository) Event(ctx context.Context, e domain.OrdersEvent) error {
	before, err := json.Marshal(e.Before)
	if err != nil {
		return err
	}
	after, err := json.Marshal(e.After)
	if err != nil {
		return err
	}
	if err = r.q.OrdersEvent(ctx, sqlc.OrdersEventParams{ID: dtos.UUID(e.ID), OrderID: dtos.UUID(e.OrderID), Action: e.Action, Reason: e.Reason, BeforeSnapshot: before, AfterSnapshot: after, At: dtos.Stamp(e.At), ActorID: dtos.UUID(e.ActorID)}); err != nil {
		return err
	}
	return r.j.Record(ctx, events.Event{ID: e.ID, Module: "purchase_orders", EntityType: "purchase_order", EntityID: e.OrderID, EntityVersion: int64(e.After.Version), Action: e.Action, ActorID: e.ActorID, ActorKind: "user", CorrelationID: e.ID, ScopeKind: "project", ScopeID: e.After.ProjectID, Visibility: "restricted", OccurredAt: e.At}, nil)
}
func (r *ordersRepository) History(ctx context.Context, id string) ([]domain.OrdersEvent, error) {
	rows, e := r.q.OrdersHistory(ctx, dtos.UUID(id))
	if e != nil {
		return nil, e
	}
	out := []domain.OrdersEvent{}
	for _, v := range rows {
		row := domain.OrdersEvent{ID: v.ID.String(), OrderID: v.SubjectID.String(), Action: v.Action, Reason: v.Reason, At: v.CreatedAt.Time.UTC(), ActorID: v.CreatedByID.String()}
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
func (r *ordersRepository) NextNumber(ctx context.Context, year int) (int64, error) {
	return r.q.OrdersNextNumber(ctx, int32(year))
}

func (r *ordersRepository) DocumentEvent(ctx context.Context, e domain.OrdersDocumentEvent) error {
	if err := r.q.OrdersDocumentEvent(ctx, sqlc.OrdersDocumentEventParams{ID: dtos.UUID(e.ID), OrderID: dtos.UUID(e.OrderID), OrderVersion: e.Version, At: dtos.Stamp(e.At), ActorID: dtos.UUID(e.ActorID)}); err != nil {
		return err
	}
	return r.j.Record(ctx, events.Event{ID: e.ID, Module: "purchase_orders", EntityType: "purchase_order_document_event", EntityID: e.ID, EntityVersion: 1, Action: "print_requested", ActorID: e.ActorID, ActorKind: "user", CorrelationID: e.ID, ScopeKind: "project", ScopeID: e.ProjectID, Visibility: "restricted", OccurredAt: e.At}, nil)
}
