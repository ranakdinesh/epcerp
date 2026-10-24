package outbound

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/application"
	"github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/ports"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	governance "github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/contracts"
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
func (Values) Now() time.Time   { return time.Now().UTC() }
func uuid(s string) pgtype.UUID { var n pgtype.UUID; _ = n.Scan(s); return n }
func pointer(n pgtype.UUID) *string {
	if !n.Valid {
		return nil
	}
	s := n.String()
	return &s
}

type AuthBridge struct{ r auth.Resolver }

func NewAuthBridge(r auth.Resolver) *AuthBridge { return &AuthBridge{r} }
func (a *AuthBridge) Resolve(ctx context.Context, t string) (domain.Actor, error) {
	p, e := a.r.Resolve(ctx, t)
	if e != nil {
		return domain.Actor{}, e
	}
	return domain.Actor{ID: p.ID, Active: p.Active, Superuser: p.Superuser, Permissions: p.Permissions}, nil
}

type UserDirectory struct{ r auth.AssignmentDirectory }

func (u *UserDirectory) Get(ctx context.Context, id string) (*domain.Actor, error) {
	a, e := u.r.Get(ctx, id)
	if e != nil || a == nil {
		return nil, e
	}
	return &domain.Actor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, nil
}

type DirectorBridge struct{ r governance.Directory }

func (d *DirectorBridge) Current(ctx context.Context) (*domain.Director, error) {
	v, e := d.r.Current(ctx)
	if e != nil || v == nil {
		return nil, e
	}
	return &domain.Director{ID: v.ID, UserID: v.UserID, Version: v.Version}, nil
}

type UnitOfWork struct {
	m *database.Manager
	u func(database.Connection) auth.AssignmentDirectory
	j func(database.Connection) events.Writer
}

func NewUnitOfWork(b database.Beginner, u func(database.Connection) auth.AssignmentDirectory, j func(database.Connection) events.Writer) *UnitOfWork {
	return &UnitOfWork{database.NewManager(b), u, j}
}
func (u *UnitOfWork) Within(ctx context.Context, f func(context.Context, ports.Repository, ports.Users) error) error {
	return u.m.Within(ctx, func(ctx context.Context, c database.Connection) error {
		return f(ctx, &repository{db.New(c), u.j(c)}, &UserDirectory{u.u(c)})
	})
}

type writer struct {
	c database.Connection
	r ports.Repository
	d ports.DirectorDirectory
	v ports.Values
}

func NewWriter(c database.Connection, d governance.Directory, j events.Writer, v ports.Values) contracts.Writer {
	return &writer{c, &repository{db.New(c), j}, &DirectorBridge{d}, v}
}
func (w *writer) DirectorDecision(ctx context.Context, eventID string) (domain.Notice, error) {
	if !database.IsTransactionConnection(ctx, w.c) {
		return domain.Notice{}, errors.New("Notice delivery requires the consuming Finance transaction")
	}
	return application.Deliver(ctx, w.r, w.d, w.v, eventID)
}

type repository struct {
	q *db.Queries
	j events.Writer
}

func mapNotice(n db.CommunicationNotice) domain.Notice {
	return domain.Notice{SourceActionID: n.SourceActionID.String(), Kind: n.NoticeKind, Title: n.Title, Link: n.Link, ID: n.ID.String(), EventID: n.SourceEventID.String(), RecipientID: n.RecipientID.String(), DirectorID: n.DirectorDesignationID.String(), DirectorVersion: n.DirectorVersion, EntityID: n.DecisionID.String(), ProjectID: n.ProjectID.String(), SourceVersion: n.SourceVersion, Action: n.DecisionAction, Version: n.Version, Active: n.IsActive, ReadAt: datePointer(n.ReadAt), AcknowledgedAt: datePointer(n.AcknowledgedAt), CreatedAt: n.CreatedAt.Time, UpdatedAt: n.UpdatedAt.Time, CreatedBy: pointer(n.CreatedByID), UpdatedBy: pointer(n.UpdatedByID)}
}
func datePointer(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	return &v.Time
}
func date(v *time.Time) pgtype.Timestamptz {
	if v == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *v, Valid: true}
}
func (r *repository) Source(ctx context.Context, id string) (*domain.Source, error) {
	v, e := r.q.NoticeSource(ctx, uuid(id))
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	return &domain.Source{EventID: v.ID.String(), EntityID: v.EntityID.String(), ProjectID: v.ScopeID.String(), ActorID: v.ActorID.String(), Module: v.SourceModule, EntityType: v.EntityType, Action: v.Action, ScopeKind: v.ScopeKind, Visibility: v.Visibility, Version: v.EntityVersion, At: v.OccurredAt.Time}, nil
}
func (r *repository) BySource(ctx context.Context, id string) (*domain.Notice, error) {
	v, e := r.q.NoticeBySource(ctx, uuid(id))
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	n := mapNotice(v)
	return &n, nil
}
func (r *repository) Create(ctx context.Context, n domain.Notice) (domain.Notice, error) {
	if n.Kind == "" {
		n.Kind = "director"
	}
	v, e := r.q.CreateNotice(ctx, db.CreateNoticeParams{ID: uuid(n.ID), SourceEventID: uuid(n.EventID), RecipientID: uuid(n.RecipientID), DirectorDesignationID: uuid(n.DirectorID), DirectorVersion: n.DirectorVersion, DecisionID: uuid(n.EntityID), ProjectID: uuid(n.ProjectID), SourceVersion: n.SourceVersion, DecisionAction: n.Action, CreatedAt: pgtype.Timestamptz{Time: n.CreatedAt, Valid: true}, CreatedByID: uuid(*n.CreatedBy), SourceActionID: uuid(n.SourceActionID), NoticeKind: n.Kind, Title: n.Title, Link: n.Link})
	if errors.Is(e, pgx.ErrNoRows) {
		row, e := r.q.NoticeByRecipientSource(ctx, db.NoticeByRecipientSourceParams{SourceEventID: uuid(n.EventID), RecipientID: uuid(n.RecipientID)})
		previous := mapNotice(row)
		if e != nil {
			return domain.Notice{}, e
		}
		if previous.ID == "" {
			return domain.Notice{}, domain.SourceConflict
		}
		return previous, nil
	}
	if e != nil {
		return domain.Notice{}, e
	}
	return mapNotice(v), nil
}
func (r *repository) Get(ctx context.Context, id, recipient string) (*domain.Notice, error) {
	v, e := r.q.OwnNotice(ctx, db.OwnNoticeParams{ID: uuid(id), RecipientID: uuid(recipient)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	n := mapNotice(v)
	return &n, nil
}
func (r *repository) List(ctx context.Context, id string, q domain.Query) ([]domain.Notice, int64, error) {
	total, e := r.q.OwnNoticeCount(ctx, db.OwnNoticeCountParams{RecipientID: uuid(id), Status: q.Status})
	if e != nil {
		return nil, 0, e
	}
	rows, e := r.q.OwnNotices(ctx, db.OwnNoticesParams{RecipientID: uuid(id), Status: q.Status, PageSize: q.Size, PageOffset: (q.Page - 1) * q.Size})
	if e != nil {
		return nil, 0, e
	}
	out := []domain.Notice{}
	for _, r := range rows {
		out = append(out, mapNotice(r))
	}
	return out, total, nil
}
func (r *repository) Save(ctx context.Context, n domain.Notice) (domain.Notice, error) {
	v, e := r.q.SaveReceipt(ctx, db.SaveReceiptParams{ID: uuid(n.ID), RecipientID: uuid(n.RecipientID), ReadAt: date(n.ReadAt), AcknowledgedAt: date(n.AcknowledgedAt), UpdatedAt: pgtype.Timestamptz{Time: n.UpdatedAt, Valid: true}, ExpectedVersion: n.Version - 1})
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Notice{}, domain.Stale
	}
	if e != nil {
		return domain.Notice{}, e
	}
	return mapNotice(v), nil
}
func (r *repository) Event(ctx context.Context, v domain.Event) error {
	before, e := json.Marshal(v.Before)
	if e != nil {
		return e
	}
	after, e := json.Marshal(v.After)
	if e != nil {
		return e
	}
	if e = r.q.CreateNoticeEvent(ctx, db.CreateNoticeEventParams{ID: uuid(v.ID), NoticeID: uuid(v.After.ID), Action: v.Action, NoticeVersion: v.After.Version, BeforeSnapshot: before, AfterSnapshot: after, CreatedAt: pgtype.Timestamptz{Time: v.At, Valid: true}, CreatedByID: uuid(v.ActorID)}); e != nil {
		return e
	}
	scope := "project"
	if v.After.Kind == "supplier_review" || v.After.Kind == "tender_manager" {
		scope = "platform"
	}
	return r.j.Record(ctx, events.Event{ID: v.ID, Module: "communication_inbox", EntityType: "director_notice", EntityID: v.After.ID, EntityVersion: int64(v.After.Version), Action: v.Action, ActorID: v.ActorID, ActorKind: "user", CorrelationID: v.After.EventID, CausationID: v.After.EventID, ScopeKind: scope, ScopeID: v.After.ProjectID, Visibility: "restricted", OccurredAt: v.At}, nil)
}
func (r *repository) History(ctx context.Context, id, recipient string) ([]domain.Event, error) {
	rows, e := r.q.OwnNoticeHistory(ctx, db.OwnNoticeHistoryParams{ID: uuid(id), RecipientID: uuid(recipient)})
	if e != nil {
		return nil, e
	}
	out := []domain.Event{}
	for _, row := range rows {
		v := domain.Event{ID: row.ID.String(), Action: row.Action, ActorID: row.CreatedByID.String(), At: row.CreatedAt.Time}
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

func NewAddressedWriter(c database.Connection, j events.Writer, v ports.Values) contracts.AddressedWriter {
	return &writer{c: c, r: &repository{db.New(c), j}, v: v}
}
func (w *writer) QuotationNotice(ctx context.Context, event, recipient, title, link string) error {
	if !database.IsTransactionConnection(ctx, w.c) {
		return errors.New("Addressed notice requires its source transaction")
	}
	return application.DeliverQuotation(ctx, w.r, w.v, event, recipient, title, link)
}

func NewSupplierReviewWriter(c database.Connection, j events.Writer, v ports.Values) contracts.SupplierReviewWriter {
	return &writer{c: c, r: &repository{db.New(c), j}, v: v}
}
func (w *writer) SupplierReviewNotice(ctx context.Context, event, recipient, title, link string) error {
	if !database.IsTransactionConnection(ctx, w.c) {
		return errors.New("Supplier notice requires its source transaction")
	}
	return application.DeliverSupplierReview(ctx, w.r, w.v, event, recipient, title, link)
}
