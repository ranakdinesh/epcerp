package outbound

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
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
func uuid(s string) pgtype.UUID { var v pgtype.UUID; _ = v.Scan(s); return v }
func actor(v pgtype.UUID) *string {
	if !v.Valid {
		return nil
	}
	s := v.String()
	return &s
}
func record(v db.TenderTenderFmsIntake) domain.Intake {
	return domain.Intake{RecordKind: v.RecordKind, ID: v.ID.String(), Title: v.Title, OwnerID: v.OwnerID.String(), Version: v.Version, Active: v.IsActive, CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time, CreatedBy: actor(v.CreatedByID), UpdatedBy: actor(v.UpdatedByID), Documents: []domain.Source{}}
}

type repository struct {
	users auth.AssignmentDirectory
	q     *db.Queries
	e     events.Writer
}

func (r *repository) OwnerActive(ctx context.Context, id string) (bool, error) {
	if r.users == nil {
		return false, errors.New("live owner directory required")
	}
	u, e := r.users.Get(ctx, id)
	return u != nil && u.Active, e
}
func (r *repository) Lock(ctx context.Context) error { return r.q.LockIntakeUploads(ctx) }
func (r *repository) List(ctx context.Context, owner string) ([]domain.Intake, error) {
	rows, e := r.q.ListIntakes(ctx, owner)
	out := []domain.Intake{}
	for _, v := range rows {
		out = append(out, record(v))
	}
	return out, e
}
func (r *repository) Get(ctx context.Context, id string) (domain.Intake, error) {
	v, e := r.q.GetIntake(ctx, uuid(id))
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Intake{}, domain.Missing
	}
	return record(v), e
}
func (r *repository) FindInitial(ctx context.Context, owner, hash string) (*domain.Intake, error) {
	v, e := r.q.FindIntake(ctx, db.FindIntakeParams{OwnerID: uuid(owner), Sha256: pgtype.Text{String: hash, Valid: true}})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	d := record(v)
	return &d, e
}
func (r *repository) Create(ctx context.Context, id, title, actor, hash string) (domain.Intake, error) {
	v, e := r.q.InsertIntake(ctx, db.InsertIntakeParams{ID: uuid(id), Title: title, ActorID: uuid(actor), Sha256: pgtype.Text{String: hash, Valid: true}})
	return record(v), e
}
func (r *repository) Changed(ctx context.Context, d domain.Intake, doc, actor, action string) error {
	if action == "source_attached" {
		exists, e := r.q.ExistingAttachment(ctx, db.ExistingAttachmentParams{IntakeID: uuid(d.ID), DocumentID: uuid(doc)})
		if e != nil {
			return e
		}
		if exists {
			return nil
		}
	}
	v, e := r.q.UpdateIntake(ctx, db.UpdateIntakeParams{ID: uuid(d.ID), ActorID: uuid(actor)})
	if e != nil {
		return e
	}
	id := Values{}.ID()
	if e = r.q.InsertIntakeEvent(ctx, db.InsertIntakeEventParams{ID: uuid(id), IntakeID: uuid(d.ID), DocumentID: uuid(doc), ActorID: uuid(actor), Action: action}); e != nil {
		return e
	}
	return r.e.Record(ctx, events.Event{ID: id, Module: "tender_fms", EntityType: "tender_intake", EntityID: d.ID, EntityVersion: int64(v.Version), Action: action, ActorID: actor, ActorKind: "user", CorrelationID: id, ScopeKind: "platform", Visibility: "restricted", OccurredAt: v.UpdatedAt.Time}, nil)
}
func (r *repository) History(ctx context.Context, id string) ([]domain.Event, error) {
	rows, e := r.q.IntakeHistory(ctx, uuid(id))
	out := []domain.Event{}
	for _, v := range rows {
		a := ""
		if v.ActorID.Valid {
			a = v.ActorID.String()
		}
		out = append(out, domain.Event{ID: v.ID.String(), DocumentID: v.DocumentID.String(), ActorID: a, Action: v.Action, CreatedAt: v.CreatedAt.Time, Version: v.Version, Reason: v.Reason})
	}
	return out, e
}

type UnitOfWork struct {
	users func(database.Connection) auth.AssignmentDirectory
	m     *database.Manager
	d     func(database.Connection) ports.Documents
	e     func(database.Connection) events.Writer
}

func NewUnitOfWork(b database.Beginner, d func(database.Connection) ports.Documents, e func(database.Connection) events.Writer) *UnitOfWork {
	return &UnitOfWork{m: database.NewManager(b), d: d, e: e}
}
func (u *UnitOfWork) WithUsers(users func(database.Connection) auth.AssignmentDirectory) *UnitOfWork {
	u.users = users
	return u
}
func (u *UnitOfWork) Within(ctx context.Context, w func(context.Context, ports.Repository, ports.Documents) error) error {
	return u.m.Within(ctx, func(ctx context.Context, c database.Connection) error {
		var users auth.AssignmentDirectory
		if u.users != nil {
			users = u.users(c)
		}
		return w(ctx, &repository{q: db.New(c), e: u.e(c), users: users}, u.d(c))
	})
}

type AuthBridge struct{ r auth.Resolver }

func NewAuthBridge(r auth.Resolver) *AuthBridge { return &AuthBridge{r} }
func (b *AuthBridge) Resolve(ctx context.Context, t string) (domain.Principal, error) {
	p, e := b.r.Resolve(ctx, t)
	if p.AccountNotReady {
		return domain.Principal{}, domain.Forbidden
	}
	return domain.Principal{ID: p.ID, Active: p.Active, Superuser: p.Superuser, Permissions: p.Permissions}, e
}
