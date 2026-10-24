package outbound

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	inbox "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/ports"
	"time"
)

type Values struct{}

func (Values) ID() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic("random source unavailable")
	}
	b[6] = b[6]&15 | 64
	b[8] = b[8]&63 | 128
	s := hex.EncodeToString(b[:])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}
func (Values) Now() domain.Timestamp              { return time.Now().UTC().Truncate(time.Microsecond) }
func uuid(s string) pgtype.UUID                   { var v pgtype.UUID; _ = v.Scan(s); return v }
func stamp(t domain.Timestamp) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

type ContextFactory func(database.Connection) ports.Context
type UnitOfWork struct {
	m *database.Manager
	c ContextFactory
	j func(database.Connection) events.Writer
	n func(database.Connection) inbox.Writer
}

func NewUnitOfWork(b database.Beginner, c ContextFactory, j func(database.Connection) events.Writer, n func(database.Connection) inbox.Writer) *UnitOfWork {
	return &UnitOfWork{database.NewManager(b), c, j, n}
}
func (u *UnitOfWork) Within(ctx context.Context, work func(context.Context, ports.Repository, ports.Context) error) error {
	return u.m.Within(ctx, func(ctx context.Context, c database.Connection) error {
		return work(ctx, &repository{db.New(c), u.j(c), u.n(c)}, u.c(c))
	})
}

type repository struct {
	q *db.Queries
	j events.Writer
	n inbox.Writer
}

var _ ports.Repository = (*repository)(nil)

func record(v db.FinanceAccountsAcceptance) (domain.Record, error) {
	var out domain.Record
	if e := json.Unmarshal(v.Data, &out); e != nil {
		return out, e
	}
	out.ID = v.ID.String()
	out.ProjectID = v.ProjectID.String()
	out.PackageID = v.PackageID.String()
	out.Version = v.Version
	out.Revision = v.Revision
	out.Acceptance.Status = v.Status
	out.Active = v.IsActive
	out.CreatedAt = v.CreatedAt.Time.UTC()
	out.UpdatedAt = v.UpdatedAt.Time.UTC()
	out.CreatedBy = v.CreatedByID.String()
	out.UpdatedBy = v.UpdatedByID.String()
	e := json.Unmarshal(v.SourceSnapshot, &out.Source)
	return out, e
}
func (r *repository) Get(ctx context.Context, p, id string) (*domain.Record, error) {
	v, e := r.q.GetAcceptance(ctx, db.GetAcceptanceParams{ProjectID: uuid(p), PackageID: uuid(id)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	out, e := record(v)
	return &out, e
}
func (r *repository) Save(ctx context.Context, v domain.Record) (domain.Record, error) {
	data, e := json.Marshal(v)
	if e != nil {
		return domain.Record{}, e
	}
	source, e := json.Marshal(v.Source)
	if e != nil {
		return domain.Record{}, e
	}
	var row db.FinanceAccountsAcceptance
	if v.Version == 1 {
		row, e = r.q.InsertAcceptance(ctx, db.InsertAcceptanceParams{ID: uuid(v.ID), ProjectID: uuid(v.ProjectID), PackageID: uuid(v.PackageID), EngineerID: uuid(v.Source.EngineerID), EngineerVersion: v.Source.EngineerVersion, Revision: v.Revision, Status: v.Acceptance.Status, SourceSnapshot: source, Data: data, At: stamp(v.CreatedAt), ActorID: uuid(v.CreatedBy)})
	} else {
		row, e = r.q.UpdateAcceptance(ctx, db.UpdateAcceptanceParams{ID: uuid(v.ID), EngineerID: uuid(v.Source.EngineerID), EngineerVersion: v.Source.EngineerVersion, Revision: v.Revision, Status: v.Acceptance.Status, SourceSnapshot: source, Data: data, At: stamp(v.UpdatedAt), ActorID: uuid(v.UpdatedBy), ExpectedVersion: v.Version - 1})
	}
	var pe *pgconn.PgError
	if errors.Is(e, pgx.ErrNoRows) || errors.As(e, &pe) && pe.Code == "23505" {
		return domain.Record{}, domain.Stale
	}
	if e != nil {
		return domain.Record{}, e
	}
	return record(row)
}
func (r *repository) Event(ctx context.Context, e domain.Event) error {
	before, err := json.Marshal(e.Before)
	if err != nil {
		return err
	}
	after, err := json.Marshal(e.After)
	if err != nil {
		return err
	}
	if err = r.q.Event(ctx, db.EventParams{ID: uuid(e.ID), AcceptanceID: uuid(e.AcceptanceID), Action: e.Action, Reason: e.Reason, BeforeSnapshot: before, AfterSnapshot: after, At: stamp(e.At), ActorID: uuid(e.ActorID)}); err != nil {
		return err
	}
	kind := "accounts_acceptance"
	if domain.FinancialEvent(e) {
		kind = "financial_decision"
	}
	return r.j.Record(ctx, events.Event{ID: e.ID, Module: "finance_accounts_acceptance", EntityType: kind, EntityID: e.AcceptanceID, EntityVersion: int64(e.After.Version), Action: domain.EventAction(e), ActorID: e.ActorID, ActorKind: "user", CorrelationID: e.ID, ScopeKind: "project", ScopeID: e.After.ProjectID, Visibility: "restricted", OccurredAt: e.At}, nil)
}
func (r *repository) History(ctx context.Context, id string) ([]domain.Event, error) {
	rows, e := r.q.History(ctx, uuid(id))
	if e != nil {
		return nil, e
	}
	out := []domain.Event{}
	for _, v := range rows {
		row := domain.Event{ID: v.ID.String(), AcceptanceID: v.SubjectID.String(), Action: v.Action, Reason: v.Reason, At: v.CreatedAt.Time.UTC(), ActorID: v.CreatedByID.String()}
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

func (r *repository) Director(ctx context.Context, event string) error {
	_, e := r.n.DirectorDecision(ctx, event)
	var d *inbox.Error
	if errors.As(e, &d) {
		return &domain.Error{Status: d.Status, Field: d.Field, Message: d.Message}
	}
	return e
}
