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

type AdvancesContextFactory func(database.Connection) ports.AdvancesContext
type AdvancesFinanceFactory func(database.Connection) ports.AdvancesFinance
type AdvancesUnitOfWork struct {
	m *database.Manager
	c AdvancesContextFactory
	f AdvancesFinanceFactory
	j func(database.Connection) events.Writer
}

func NewAdvancesUnitOfWork(b database.Beginner, c AdvancesContextFactory, f AdvancesFinanceFactory, j func(database.Connection) events.Writer) *AdvancesUnitOfWork {
	return &AdvancesUnitOfWork{database.NewManager(b), c, f, j}
}
func (u *AdvancesUnitOfWork) Within(ctx context.Context, work func(context.Context, ports.AdvancesRepository, ports.AdvancesContext, ports.AdvancesFinance) error) error {
	return u.m.Within(ctx, func(ctx context.Context, c database.Connection) error {
		return work(ctx, &advancesRepository{NewStore(c), u.j(c)}, u.c(c), u.f(c))
	})
}

type advancesRepository struct {
	q *sqlc.Queries
	j events.Writer
}

var _ ports.AdvancesRepository = (*advancesRepository)(nil)

func (r *advancesRepository) Get(ctx context.Context, p, id string) (*domain.AdvancesRecord, error) {
	v, e := r.q.AdvancesGetAdvance(ctx, sqlc.AdvancesGetAdvanceParams{ProjectID: dtos.UUID(p), PackageID: dtos.UUID(id)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	out, e := dtos.AdvancesRecord(v)
	return &out, e
}
func (r *advancesRepository) Save(ctx context.Context, v domain.AdvancesRecord) (domain.AdvancesRecord, error) {
	data, e := json.Marshal(v)
	if e != nil {
		return domain.AdvancesRecord{}, e
	}
	source, e := json.Marshal(v.Source)
	if e != nil {
		return domain.AdvancesRecord{}, e
	}
	snapshot, fingerprint := "", ""
	if v.SubmittedVersion > 0 {
		snapshot, fingerprint, e = advancesSubmission(v)
		if e != nil {
			return domain.AdvancesRecord{}, e
		}
	}
	var row sqlc.PurchaseAdvance
	if v.Version == 1 {
		row, e = r.q.AdvancesInsertAdvance(ctx, sqlc.AdvancesInsertAdvanceParams{ID: dtos.UUID(v.ID), ProjectID: dtos.UUID(v.ProjectID), PackageID: dtos.UUID(v.PackageID), OrderID: dtos.UUID(v.Source.OrderID), OrderVersion: v.Source.OrderVersion, State: v.State, SourceSnapshot: source, SubmittedSnapshot: snapshot, SourceFingerprint: fingerprint, DecisionID: dtos.UUID(v.DecisionID), Data: data, At: dtos.Stamp(v.CreatedAt), ActorID: dtos.UUID(v.CreatedBy)})
	} else {
		row, e = r.q.AdvancesUpdateAdvance(ctx, sqlc.AdvancesUpdateAdvanceParams{ID: dtos.UUID(v.ID), State: v.State, SourceSnapshot: source, SubmittedSnapshot: snapshot, SourceFingerprint: fingerprint, DecisionID: dtos.UUID(v.DecisionID), Data: data, At: dtos.Stamp(v.UpdatedAt), ActorID: dtos.UUID(v.UpdatedBy), ExpectedVersion: v.Version - 1})
	}
	var pe *pgconn.PgError
	if errors.Is(e, pgx.ErrNoRows) || errors.As(e, &pe) && pe.Code == "23505" {
		return domain.AdvancesRecord{}, domain.AdvancesStale
	}
	if e != nil {
		return domain.AdvancesRecord{}, e
	}
	return dtos.AdvancesRecord(row)
}
func (r *advancesRepository) Event(ctx context.Context, e domain.AdvancesEvent) error {
	before, err := json.Marshal(e.Before)
	if err != nil {
		return err
	}
	after, err := json.Marshal(e.After)
	if err != nil {
		return err
	}
	if err = r.q.AdvancesEvent(ctx, sqlc.AdvancesEventParams{ID: dtos.UUID(e.ID), RequestID: dtos.UUID(e.RequestID), Action: e.Action, Reason: e.Reason, BeforeSnapshot: before, AfterSnapshot: after, At: dtos.Stamp(e.At), ActorID: dtos.UUID(e.ActorID)}); err != nil {
		return err
	}
	return r.j.Record(ctx, events.Event{ID: e.ID, Module: "purchase_advances", EntityType: "purchase_advance", EntityID: e.RequestID, EntityVersion: int64(e.After.Version), Action: e.Action, ActorID: e.ActorID, ActorKind: "user", CorrelationID: e.ID, ScopeKind: "project", ScopeID: e.After.ProjectID, Visibility: "restricted", OccurredAt: e.At}, nil)
}
func (r *advancesRepository) History(ctx context.Context, id string) ([]domain.AdvancesEvent, error) {
	rows, e := r.q.AdvancesHistory(ctx, dtos.UUID(id))
	if e != nil {
		return nil, e
	}
	out := []domain.AdvancesEvent{}
	for _, v := range rows {
		row := domain.AdvancesEvent{ID: v.ID.String(), RequestID: v.SubjectID.String(), Action: v.Action, Reason: v.Reason, At: v.CreatedAt.Time.UTC(), ActorID: v.CreatedByID.String()}
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

func (r *advancesRepository) Queue(ctx context.Context, offset int32) ([]domain.AdvancesRecord, error) {
	rows, e := r.q.AdvancesQueueRecords(ctx, offset)
	if e != nil {
		return nil, e
	}
	out := []domain.AdvancesRecord{}
	for _, v := range rows {
		row, e := dtos.AdvancesRecord(v)
		if e != nil {
			return nil, e
		}
		out = append(out, row)
	}
	return out, nil
}
