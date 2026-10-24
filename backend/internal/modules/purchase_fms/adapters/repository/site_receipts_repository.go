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

type SiteReceiptsContextFactory func(database.Connection) ports.SiteReceiptsContext
type SiteReceiptsUnitOfWork = unitOfWork[ports.SiteReceiptsRepository, ports.SiteReceiptsContext]

func NewSiteReceiptsUnitOfWork(b database.Beginner, c SiteReceiptsContextFactory, j func(database.Connection) events.Writer) *SiteReceiptsUnitOfWork {
	return newUnitOfWork(b, func(connection database.Connection) (ports.SiteReceiptsRepository, ports.SiteReceiptsContext) {
		return &siteReceiptsRepository{NewStore(connection), j(connection)}, c(connection)
	})
}

type siteReceiptsRepository struct {
	q *sqlc.Queries
	j events.Writer
}

var _ ports.SiteReceiptsRepository = (*siteReceiptsRepository)(nil)

func (r *siteReceiptsRepository) Get(ctx context.Context, p, id string) (*domain.SiteReceiptsRecord, error) {
	v, e := r.q.SiteReceiptsGetReceipt(ctx, sqlc.SiteReceiptsGetReceiptParams{ProjectID: dtos.UUID(p), PackageID: dtos.UUID(id)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	out, e := dtos.SiteReceiptsRecord(v)
	return &out, e
}
func (r *siteReceiptsRepository) Save(ctx context.Context, v domain.SiteReceiptsRecord) (domain.SiteReceiptsRecord, error) {
	data, e := json.Marshal(v)
	if e != nil {
		return domain.SiteReceiptsRecord{}, e
	}
	source, e := json.Marshal(v.Source)
	if e != nil {
		return domain.SiteReceiptsRecord{}, e
	}
	var row sqlc.PurchaseSiteReceipt
	if v.Version == 1 {
		row, e = r.q.SiteReceiptsInsertReceipt(ctx, sqlc.SiteReceiptsInsertReceiptParams{ID: dtos.UUID(v.ID), ProjectID: dtos.UUID(v.ProjectID), PackageID: dtos.UUID(v.PackageID), DispatchID: dtos.UUID(v.Source.DispatchID), DispatchVersion: v.Source.DispatchVersion, Revision: v.Revision, Status: v.Receipt.Status, SourceSnapshot: source, Data: data, At: dtos.Stamp(v.CreatedAt), ActorID: dtos.UUID(v.CreatedBy)})
	} else {
		row, e = r.q.SiteReceiptsUpdateReceipt(ctx, sqlc.SiteReceiptsUpdateReceiptParams{ID: dtos.UUID(v.ID), DispatchID: dtos.UUID(v.Source.DispatchID), DispatchVersion: v.Source.DispatchVersion, Revision: v.Revision, Status: v.Receipt.Status, SourceSnapshot: source, Data: data, At: dtos.Stamp(v.UpdatedAt), ActorID: dtos.UUID(v.UpdatedBy), ExpectedVersion: v.Version - 1})
	}
	var pe *pgconn.PgError
	if errors.Is(e, pgx.ErrNoRows) || errors.As(e, &pe) && pe.Code == "23505" {
		return domain.SiteReceiptsRecord{}, domain.SiteReceiptsStale
	}
	if e != nil {
		return domain.SiteReceiptsRecord{}, e
	}
	return dtos.SiteReceiptsRecord(row)
}
func (r *siteReceiptsRepository) Event(ctx context.Context, e domain.SiteReceiptsEvent) error {
	before, err := json.Marshal(e.Before)
	if err != nil {
		return err
	}
	after, err := json.Marshal(e.After)
	if err != nil {
		return err
	}
	if err = r.q.SiteReceiptsEvent(ctx, sqlc.SiteReceiptsEventParams{ID: dtos.UUID(e.ID), ReceiptID: dtos.UUID(e.ReceiptID), Action: e.Action, Reason: e.Reason, BeforeSnapshot: before, AfterSnapshot: after, At: dtos.Stamp(e.At), ActorID: dtos.UUID(e.ActorID)}); err != nil {
		return err
	}
	return r.j.Record(ctx, events.Event{ID: e.ID, Module: "purchase_site_receipts", EntityType: "purchase_site_receipts", EntityID: e.ReceiptID, EntityVersion: int64(e.After.Version), Action: e.Action, ActorID: e.ActorID, ActorKind: "user", CorrelationID: e.ID, ScopeKind: "project", ScopeID: e.After.ProjectID, Visibility: "restricted", OccurredAt: e.At}, nil)
}
func (r *siteReceiptsRepository) History(ctx context.Context, id string) ([]domain.SiteReceiptsEvent, error) {
	rows, e := r.q.SiteReceiptsHistory(ctx, dtos.UUID(id))
	if e != nil {
		return nil, e
	}
	out := []domain.SiteReceiptsEvent{}
	for _, v := range rows {
		row := domain.SiteReceiptsEvent{ID: v.ID.String(), ReceiptID: v.SubjectID.String(), Action: v.Action, Reason: v.Reason, At: v.CreatedAt.Time.UTC(), ActorID: v.CreatedByID.String()}
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
