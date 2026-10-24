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
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/application"
	"github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/ports"
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

func users(r auth.AssignmentDirectory) *UserDirectory { return &UserDirectory{r} }
func mapUser(v auth.AssignmentCandidate) domain.User {
	return domain.User{ID: v.ID, Name: v.Name, Active: v.Active, Superuser: v.Superuser, Permissions: v.Permissions}
}
func (u *UserDirectory) Get(ctx context.Context, id string) (*domain.User, error) {
	v, e := u.r.Get(ctx, id)
	if e != nil || v == nil {
		return nil, e
	}
	out := mapUser(*v)
	return &out, nil
}
func (u *UserDirectory) List(ctx context.Context) ([]domain.User, error) {
	rows, e := u.r.List(ctx)
	if e != nil {
		return nil, e
	}
	if len(rows) > 1000 {
		return nil, domain.Invalid("users", "Director account choices exceed 1000; use a bounded administrator directory")
	}
	out := []domain.User{}
	for _, row := range rows {
		if row.Active {
			out = append(out, mapUser(row))
		}
	}
	return out, nil
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
		return f(ctx, &repository{db.New(c), u.j(c)}, users(u.u(c)))
	})
}

type repository struct {
	q *db.Queries
	j events.Writer
}

func (r *repository) Lock(ctx context.Context) error { return r.q.LockDirectorSlot(ctx) }
func mapRecord(v db.OrganisationDirectorDesignation) domain.Designation {
	return domain.Designation{ID: v.ID.String(), UserID: v.UserID.String(), Version: v.Version, Active: v.IsActive, CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time, CreatedBy: pointer(v.CreatedByID), UpdatedBy: pointer(v.UpdatedByID)}
}
func (r *repository) Get(ctx context.Context) (*domain.Designation, error) {
	v, e := r.q.GetDirector(ctx)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	out := mapRecord(v)
	return &out, nil
}
func (r *repository) Save(ctx context.Context, v domain.Designation) (domain.Designation, error) {
	var row db.OrganisationDirectorDesignation
	var e error
	actor := pgtype.UUID{}
	if v.UpdatedBy != nil {
		actor = uuid(*v.UpdatedBy)
	}
	if v.Version == 1 {
		row, e = r.q.CreateDirector(ctx, db.CreateDirectorParams{ID: uuid(v.ID), UserID: uuid(v.UserID), IsActive: v.Active, CreatedByID: actor})
	} else {
		row, e = r.q.UpdateDirector(ctx, db.UpdateDirectorParams{UserID: uuid(v.UserID), IsActive: v.Active, ActorID: actor, ExpectedVersion: v.Version - 1})
	}
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Designation{}, domain.Stale
	}
	return mapRecord(row), e
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
	if e = r.q.AddDirectorEvent(ctx, db.AddDirectorEventParams{ID: uuid(v.ID), DesignationID: uuid(v.After.ID), Action: v.Action, Reason: v.Reason, BeforeSnapshot: before, AfterSnapshot: after, CreatedAt: pgtype.Timestamptz{Time: v.At, Valid: true}, CreatedByID: uuid(v.ActorID)}); e != nil {
		return e
	}
	return r.j.Record(ctx, events.Event{ID: v.ID, Module: "organisation_governance", EntityType: "director_designation", EntityID: v.After.ID, EntityVersion: int64(v.After.Version), Action: v.Action, ActorID: v.ActorID, ActorKind: "user", CorrelationID: v.ID, ScopeKind: "platform", Visibility: "restricted", OccurredAt: v.At}, nil)
}
func (r *repository) History(ctx context.Context) ([]domain.Event, error) {
	rows, e := r.q.DirectorHistory(ctx)
	if e != nil {
		return nil, e
	}
	out := []domain.Event{}
	for _, row := range rows {
		v := domain.Event{ID: row.ID.String(), Action: row.Action, Reason: row.Reason, ActorID: row.CreatedByID.String(), At: row.CreatedAt.Time}
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

type directory struct {
	c database.Connection
	u ports.Users
	r ports.Repository
}

func NewDirectory(c database.Connection, u auth.AssignmentDirectory) contracts.Directory {
	return &directory{c, users(u), &repository{q: db.New(c)}}
}
func (d *directory) Current(ctx context.Context) (*domain.Designation, error) {
	if !database.IsTransactionConnection(ctx, d.c) {
		return nil, errors.New("Director directory requires the consuming transaction")
	}
	return application.Recipient(ctx, d.r, d.u)
}
