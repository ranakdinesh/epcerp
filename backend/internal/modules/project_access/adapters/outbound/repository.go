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
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/project_access/application"
	"github.com/ranakdinesh/epcERP/internal/modules/project_access/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/project_access/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/project_access/ports"
	projects "github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"
	purchaseContracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	"strings"
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
func uuid(s string) pgtype.UUID { var v pgtype.UUID; _ = v.Scan(s); return v }
func actor(v pgtype.UUID) *string {
	if !v.Valid {
		return nil
	}
	s := v.String()
	return &s
}

type AuthBridge struct{ r auth.Resolver }

func NewAuthBridge(r auth.Resolver) *AuthBridge { return &AuthBridge{r} }
func (a *AuthBridge) Resolve(ctx context.Context, t string) (domain.Actor, error) {
	p, e := a.r.Resolve(ctx, t)
	return domain.Actor{ID: p.ID, Active: p.Active, Superuser: p.Superuser, Permissions: p.Permissions}, e
}

type Providers struct {
	Projects projects.ContextReader
	Purchase purchaseContracts.WorkspaceOwnership
	Users    auth.AssignmentDirectory
	Bundles  auth.BundleDirectory
}
type Factory func(database.Connection) Providers
type UnitOfWork struct {
	m *database.Manager
	p Factory
	j func(database.Connection) events.Writer
}

func NewUnitOfWork(b database.Beginner, p Factory, j func(database.Connection) events.Writer) *UnitOfWork {
	return &UnitOfWork{database.NewManager(b), p, j}
}
func (u *UnitOfWork) Within(ctx context.Context, f func(context.Context, ports.Repository, ports.Context) error) error {
	return u.m.Within(ctx, func(ctx context.Context, c database.Connection) error {
		return f(ctx, &repository{db.New(c), u.j(c)}, &sourceContext{u.p(c)})
	})
}
func NewGate(c database.Connection, p Providers) contracts.Gate {
	return &readGate{c, application.NewGate(&repository{q: db.New(c)}, &sourceContext{p})}
}

type readGate struct {
	c database.Connection
	g *application.Gate
}

func (g *readGate) Check(ctx context.Context, a domain.Actor, p, code string, write bool) (domain.Access, error) {
	if !database.IsTransactionConnection(ctx, g.c) {
		return domain.Access{}, errors.New("project access gate requires the consumer transaction")
	}
	return g.g.Check(ctx, a, p, code, write)
}
func (g *readGate) Owners(ctx context.Context, p string) ([]domain.Owner, error) {
	if !database.IsTransactionConnection(ctx, g.c) {
		return nil, errors.New("project owners require the consumer transaction")
	}
	return g.g.Owners(ctx, p)
}

type sourceContext struct{ p Providers }

func (c *sourceContext) Project(ctx context.Context, id string, lock bool) (*domain.Project, error) {
	p, e := c.p.Projects.Get(ctx, id, lock)
	if e != nil || p == nil {
		return nil, e
	}
	pe, e := c.p.Purchase.Owner(ctx, id)
	if e != nil {
		return nil, e
	}
	return &domain.Project{ID: p.ID, Name: p.Name, ManagerID: p.ManagerID, ProcurementExecutiveID: pe, Active: p.Active, Lifecycle: p.Lifecycle}, nil
}
func (c *sourceContext) User(ctx context.Context, id string) (*domain.User, error) {
	u, e := c.p.Users.Get(ctx, id)
	if e != nil || u == nil {
		return nil, e
	}
	return &domain.User{ID: u.ID, Name: u.Name, Active: u.Active, Superuser: u.Superuser, Permissions: u.Permissions}, nil
}
func (c *sourceContext) Bundle(ctx context.Context, id string) (*domain.Bundle, error) {
	b, e := c.p.Bundles.Get(ctx, id)
	if e != nil || b == nil {
		return nil, e
	}
	return &domain.Bundle{ID: b.ID, Name: b.Name, Active: b.Active, Permissions: b.Permissions}, nil
}
func (c *sourceContext) Member(ctx context.Context, u, b string) (bool, error) {
	return c.p.Bundles.Member(ctx, u, b)
}
func (c *sourceContext) Options(ctx context.Context) (domain.Options, error) {
	users, e := c.p.Users.List(ctx)
	if e != nil {
		return domain.Options{}, e
	}
	if len(users) > 1000 {
		return domain.Options{}, domain.Invalid("users", "Assignment directory exceeds the supported bound")
	}
	bundles, e := c.p.Bundles.List(ctx)
	if e != nil {
		return domain.Options{}, e
	}
	if len(bundles) > 1000 {
		return domain.Options{}, domain.Invalid("bundles", "Role directory exceeds the supported bound")
	}
	out := domain.Options{Users: []domain.User{}, Bundles: []domain.Bundle{}}
	for _, u := range users {
		assigned, e := c.p.Bundles.Assigned(ctx, u.ID)
		if e != nil {
			return domain.Options{}, e
		}
		ids := []string{}
		for _, b := range assigned {
			ids = append(ids, b.ID)
		}
		out.Users = append(out.Users, domain.User{ID: u.ID, Name: u.Name, Active: u.Active, Superuser: u.Superuser, Permissions: u.Permissions, BundleIDs: ids})
	}
	for _, b := range bundles {
		if !b.Active {
			continue
		}
		codes := []string{}
		for _, p := range b.Permissions {
			if domain.ScopedCode(p) {
				codes = append(codes, p)
			}
		}
		if len(codes) > 0 {
			out.Bundles = append(out.Bundles, domain.Bundle{ID: b.ID, Name: b.Name, Active: b.Active, Permissions: codes})
		}
	}
	return out, nil
}

type repository struct {
	q *db.Queries
	j events.Writer
}

func view(r db.ProjectResponsibilityProfile) (domain.Profile, error) {
	v := domain.Profile{ID: r.ID.String(), ProjectID: r.ProjectID.String(), UserID: r.UserID.String(), BundleID: r.BundleID.String(), Responsibility: r.Responsibility, Active: r.IsActive, Version: r.Version, CreatedAt: r.CreatedAt.Time, UpdatedAt: r.UpdatedAt.Time, CreatedBy: actor(r.CreatedByID), UpdatedBy: actor(r.UpdatedByID)}
	e := json.Unmarshal(r.PermissionCodes, &v.Permissions)
	return v, e
}
func (r *repository) List(ctx context.Context, p string) ([]domain.Profile, error) {
	rows, e := r.q.ListProfiles(ctx, uuid(p))
	if e != nil {
		return nil, e
	}
	if len(rows) > 1000 {
		return nil, domain.Invalid("profiles", "Project responsibility limit reached")
	}
	out := []domain.Profile{}
	for _, row := range rows {
		v, e := view(row)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}
func (r *repository) Get(ctx context.Context, p, id string) (*domain.Profile, error) {
	row, e := r.q.GetProfile(ctx, db.GetProfileParams{ProjectID: uuid(p), ID: uuid(id)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	v, e := view(row)
	return &v, e
}
func (r *repository) Save(ctx context.Context, v domain.Profile, a string) (domain.Profile, error) {
	codes, e := json.Marshal(v.Permissions)
	if e != nil {
		return v, e
	}
	var row db.ProjectResponsibilityProfile
	if v.Version == 1 {
		row, e = r.q.InsertProfile(ctx, db.InsertProfileParams{ID: uuid(v.ID), ProjectID: uuid(v.ProjectID), UserID: uuid(v.UserID), BundleID: uuid(v.BundleID), Responsibility: v.Responsibility, ResponsibilityKey: strings.ToLower(v.Responsibility), PermissionCodes: codes, ActorID: uuid(a)})
	} else {
		row, e = r.q.UpdateProfile(ctx, db.UpdateProfileParams{ID: uuid(v.ID), ProjectID: uuid(v.ProjectID), Responsibility: v.Responsibility, ResponsibilityKey: strings.ToLower(v.Responsibility), PermissionCodes: codes, IsActive: v.Active, ExpectedVersion: v.Version - 1, ActorID: uuid(a)})
	}
	if errors.Is(e, pgx.ErrNoRows) {
		return v, domain.Stale
	}
	var constraint *pgconn.PgError
	if errors.As(e, &constraint) && constraint.Code == "23505" {
		return v, domain.Duplicate
	}
	if e != nil {
		return v, e
	}
	return view(row)
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
	if e = r.q.InsertProfileEvent(ctx, db.InsertProfileEventParams{ID: uuid(v.ID), ProfileID: uuid(v.ProfileID), ProjectID: uuid(v.ProjectID), Action: v.Action, Reason: v.Reason, BeforeSnapshot: before, AfterSnapshot: after, ActorID: uuid(v.ActorID)}); e != nil {
		return e
	}
	return r.j.Record(ctx, events.Event{ID: v.ID, Module: "project_access", EntityType: "project_responsibility", EntityID: v.ProfileID, EntityVersion: int64(v.After.Version), Action: v.Action, ActorID: v.ActorID, ActorKind: "user", CorrelationID: v.ID, ScopeKind: "project", ScopeID: v.ProjectID, Visibility: "restricted", OccurredAt: v.At}, nil)
}
func (r *repository) History(ctx context.Context, p, id string) ([]domain.Event, error) {
	rows, e := r.q.ProfileHistory(ctx, db.ProfileHistoryParams{ProjectID: uuid(p), ProfileID: uuid(id)})
	if e != nil {
		return nil, e
	}
	out := []domain.Event{}
	for _, row := range rows {
		v := domain.Event{ID: row.ID.String(), ProfileID: row.ProfileID.String(), ProjectID: row.ProjectID.String(), Action: row.Action, Reason: row.Reason, ActorID: row.CreatedByID.String(), At: row.CreatedAt.Time}
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
