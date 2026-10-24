package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	inbox "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	finance "github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/contracts"
	dtos "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type QuotationApprovalsFactory func(database.Connection) QuotationApprovalsProviders
type QuotationApprovalsUnitOfWork struct {
	n func(database.Connection) inbox.AddressedWriter
	m *database.Manager
	p QuotationApprovalsFactory
	j func(database.Connection) events.Writer
	f func(database.Connection) ports.QuotationApprovalsFinance
}

func NewQuotationApprovalsUnitOfWork(b database.Beginner, p QuotationApprovalsFactory, j func(database.Connection) events.Writer, f func(database.Connection) ports.QuotationApprovalsFinance) *QuotationApprovalsUnitOfWork {
	return &QuotationApprovalsUnitOfWork{m: database.NewManager(b), p: p, j: j, f: f}
}
func (u *QuotationApprovalsUnitOfWork) Within(ctx context.Context, work func(context.Context, ports.QuotationApprovalsRepository, ports.QuotationApprovalsContext, ports.QuotationApprovalsFinance) error) error {
	return u.m.Within(ctx, func(ctx context.Context, c database.Connection) error {
		return work(ctx, &quotationApprovalsRepository{q: NewStore(c), j: u.j(c), n: func() inbox.AddressedWriter {
			if u.n == nil {
				return nil
			}
			return u.n(c)
		}()}, &quotationApprovalsSourceContext{u.p(c)}, u.f(c))
	})
}

type quotationApprovalsRepository struct {
	n inbox.AddressedWriter
	q *sqlc.Queries
	j events.Writer
}

func (r *quotationApprovalsRepository) Get(ctx context.Context, p, b string) (*domain.QuotationApprovalsRecord, error) {
	v, e := r.q.QuotationApprovalsGetRecord(ctx, sqlc.QuotationApprovalsGetRecordParams{ProjectID: dtos.UUID(p), BoID: dtos.UUID(b)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	out, e := dtos.QuotationApprovalsRecord(v)
	return &out, e
}
func (r *quotationApprovalsRepository) Save(ctx context.Context, v domain.QuotationApprovalsRecord, actorID string) (domain.QuotationApprovalsRecord, error) {
	data, e := json.Marshal(v.Attempts)
	if e != nil {
		return domain.QuotationApprovalsRecord{}, e
	}
	var row sqlc.QuotationApprovalSet
	if v.Version == 1 {
		row, e = r.q.QuotationApprovalsInsertRecord(ctx, sqlc.QuotationApprovalsInsertRecordParams{ProjectID: dtos.UUID(v.ProjectID), BoID: dtos.UUID(v.BOID), Attempts: data, ActorID: dtos.UUID(actorID)})
	} else {
		row, e = r.q.QuotationApprovalsUpdateRecord(ctx, sqlc.QuotationApprovalsUpdateRecordParams{ProjectID: dtos.UUID(v.ProjectID), BoID: dtos.UUID(v.BOID), Attempts: data, ActorID: dtos.UUID(actorID), ExpectedVersion: v.Version - 1})
	}
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.QuotationApprovalsRecord{}, domain.QuotationApprovalsStale
	}
	if e != nil {
		return domain.QuotationApprovalsRecord{}, e
	}
	return dtos.QuotationApprovalsRecord(row)
}
func (r *quotationApprovalsRepository) Event(ctx context.Context, e domain.QuotationApprovalsEvent) error {
	before, err := json.Marshal(e.Before)
	if err != nil {
		return err
	}
	after, err := json.Marshal(e.After)
	if err != nil {
		return err
	}
	if err = r.q.QuotationApprovalsEvent(ctx, sqlc.QuotationApprovalsEventParams{ID: dtos.UUID(e.ID), BoID: dtos.UUID(e.BOID), Action: e.Action, Reason: e.Reason, BeforeSnapshot: before, AfterSnapshot: after, CreatedAt: pgtype.Timestamptz{Time: e.CreatedAt, Valid: true}, ActorID: dtos.UUID(e.ActorID)}); err != nil {
		return err
	}
	return r.j.Record(ctx, events.Event{ID: e.ID, Module: "quotation_approvals", EntityType: "quotation_approval_set", EntityID: e.BOID, EntityVersion: int64(e.After.Version), Action: e.Action, ActorID: e.ActorID, ActorKind: "user", CorrelationID: e.ID, ScopeKind: "project", ScopeID: e.ProjectID, Visibility: "restricted", OccurredAt: e.CreatedAt}, nil)
}
func (r *quotationApprovalsRepository) History(ctx context.Context, p, b string) ([]domain.QuotationApprovalsEvent, error) {
	rows, e := r.q.QuotationApprovalsHistory(ctx, sqlc.QuotationApprovalsHistoryParams{ProjectID: dtos.UUID(p), BoID: dtos.UUID(b)})
	if e != nil {
		return nil, e
	}
	out := []domain.QuotationApprovalsEvent{}
	for _, row := range rows {
		v := domain.QuotationApprovalsEvent{ID: row.ID.String(), ProjectID: p, BOID: b, Action: row.Action, Reason: row.Reason, ActorID: row.CreatedByID.String(), CreatedAt: row.CreatedAt.Time}
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

type QuotationApprovalsFinanceBridge struct{ f finance.QuotationReviewer }

func NewQuotationApprovalsFinanceBridge(f finance.QuotationReviewer) *QuotationApprovalsFinanceBridge {
	return &QuotationApprovalsFinanceBridge{f}
}
func (b *QuotationApprovalsFinanceBridge) Decide(ctx context.Context, a domain.Principal, r domain.QuotationApprovalsRecord, attempt domain.QuotationApprovalsAttempt, reason string) (string, error) {
	snapshot, e := json.Marshal(attempt)
	if e != nil {
		return "", e
	}
	hash := sha256.Sum256(snapshot)
	q := finance.QuotationRequest{ProjectID: r.ProjectID, BOID: r.BOID, AttemptID: attempt.ID, SubmittedBy: attempt.SubmittedBy, Outcome: attempt.State, Version: r.Version, Amount: attempt.Total, Currency: "", Fingerprint: hex.EncodeToString(hash[:]), Reason: reason} // Currency comes from the saved financial snapshot, never a browser command.
	q.Currency = attempt.Currency
	id, e := b.f.ReviewQuotation(ctx, finance.Actor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, q)
	var d *finance.Error
	if errors.As(e, &d) {
		return "", &domain.QuotationApprovalsError{Status: d.Status, Field: d.Field, Message: d.Message}
	}
	return id, e
}

func (r *quotationApprovalsRepository) Queue(ctx context.Context, offset int32) ([]domain.QuotationApprovalsRecord, error) {
	rows, e := r.q.QuotationApprovalsQueueRecords(ctx, offset)
	if e != nil {
		return nil, e
	}
	out := []domain.QuotationApprovalsRecord{}
	for _, v := range rows {
		out = append(out, domain.QuotationApprovalsRecord{ProjectID: v.ProjectID.String(), BOID: v.BoID.String(), Version: v.Version, UpdatedAt: v.UpdatedAt.Time, Active: true})
	}
	return out, nil
}

func (u *QuotationApprovalsUnitOfWork) WithNotices(n func(database.Connection) inbox.AddressedWriter) *QuotationApprovalsUnitOfWork {
	u.n = n
	return u
}
func (r *quotationApprovalsRepository) Notify(ctx context.Context, p domain.QuotationWorkPlan) error {
	if r.n == nil {
		return domain.QuotationApprovalsInvalid("notifications", "Quotation notice delivery unavailable")
	}
	for _, n := range p.Notices {
		if err := r.n.QuotationNotice(ctx, p.EventID, n.Recipient, n.Title, n.Link); err != nil {
			return err
		}
	}
	if p.ResolveGroup {
		if err := r.q.QuotationWorkResolveEarlier(ctx, sqlc.QuotationWorkResolveEarlierParams{BoID: dtos.UUID(p.BOID), GroupID: dtos.UUID(p.GroupID), ActorID: dtos.UUID(p.ActorID)}); err != nil {
			return err
		}
	}
	if p.ResolveAttempt {
		if err := r.q.QuotationWorkResolve(ctx, sqlc.QuotationWorkResolveParams{AttemptID: dtos.UUID(p.AttemptID), ActorID: dtos.UUID(p.ActorID)}); err != nil {
			return err
		}
	}
	for _, entry := range p.Create {
		if err := r.q.QuotationWorkCreate(ctx, sqlc.QuotationWorkCreateParams{ID: dtos.UUID(entry.ID), ProjectID: dtos.UUID(p.ProjectID), BoID: dtos.UUID(p.BOID), GroupID: dtos.UUID(p.GroupID), AttemptID: dtos.UUID(p.AttemptID), RecipientID: dtos.UUID(entry.Recipient), Kind: entry.Kind, State: entry.State, Title: entry.Title, Link: entry.Link, ActorID: dtos.UUID(p.ActorID)}); err != nil {
			return err
		}
	}
	if p.Return != nil {
		return r.q.QuotationWorkReturn(ctx, sqlc.QuotationWorkReturnParams{AttemptID: dtos.UUID(p.AttemptID), ActorID: dtos.UUID(p.ActorID), State: p.Return.State, Title: p.Return.Title, Link: p.Return.Link})
	}
	return nil
}

func (r *quotationApprovalsRepository) ProjectRecords(ctx context.Context, p string) ([]domain.QuotationApprovalsRecord, error) {
	rows, e := r.q.QuotationProjectRecords(ctx, dtos.UUID(p))
	if e != nil {
		return nil, e
	}
	out := []domain.QuotationApprovalsRecord{}
	for _, row := range rows {
		v, e := dtos.QuotationApprovalsRecord(row)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}
