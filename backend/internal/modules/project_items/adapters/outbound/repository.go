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
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/project_items/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/project_items/application"
	"github.com/ranakdinesh/epcERP/internal/modules/project_items/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/project_items/ports"
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

type Commitments interface {
	Committed(context.Context, string, string) (string, error)
}
type Providers struct {
	Scope       purchaseContracts.WorkspaceAccessGate
	Items       masterContracts.ItemReader
	Rules       purchaseContracts.ClientRulesEvaluator
	Projects    projects.ContextReader
	Commitments Commitments
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
		return f(ctx, &repository{db.New(c), u.j(c)}, &itemContext{u.p(c)})
	})
}

type itemContext struct{ p Providers }

func (c *itemContext) Scope(ctx context.Context, a domain.Actor, project, permission string, write bool) error {
	_, e := c.p.Scope.Check(ctx, purchaseContracts.WorkspacePrincipal{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, project, permission, write)
	if errors.Is(e, purchaseContracts.WorkspaceErrUnauthorized) {
		return domain.Unauthorized
	}
	if errors.Is(e, purchaseContracts.WorkspaceErrForbidden) {
		return domain.Forbidden
	}
	if errors.Is(e, purchaseContracts.WorkspaceErrMissing) {
		return domain.Missing
	}
	var invalid *purchaseContracts.WorkspaceInvalid
	if errors.As(e, &invalid) {
		return domain.Invalid(invalid.Field, invalid.Message)
	}
	return e
}
func (c *itemContext) Item(ctx context.Context, id string, approved bool) (domain.Snapshot, error) {
	var item masterContracts.Item
	var e error
	if approved {
		item, e = c.p.Items.Approved(ctx, id)
	} else {
		item, e = c.p.Items.Get(ctx, id)
	}
	if e != nil {
		var source *masterContracts.Error
		if errors.As(e, &source) {
			status := 500
			switch source.Kind {
			case "unauthenticated":
				status = 401
			case "access_denied":
				status = 403
			case "not_found":
				status = 404
			case "conflict":
				status = 409
			case "invalid_input":
				status = 422
			}
			return domain.Snapshot{}, &domain.Error{Status: status, Field: "item_id", Message: source.Detail}
		}
		return domain.Snapshot{}, e
	}
	config := item.Snapshot.Configuration
	return domain.Snapshot{ID: item.ID, SKU: item.SKU, Description: item.Description, EngineeringKey: item.EngineeringKey, UOM: config.UOM, Manufacturer: item.Snapshot.Manufacturer.Name, ManufacturerID: item.ManufacturerID, CategoryCode: config.CategoryCode, FamilyCode: config.FamilyCode, ConfigurationCode: config.Code}, nil
}
func (c *itemContext) Policy(ctx context.Context, project string, item domain.Snapshot) error {
	p, e := c.p.Projects.Get(ctx, project, false)
	if e != nil {
		return e
	}
	if p == nil {
		return domain.Missing
	}
	v, e := c.p.Rules.Evaluate(ctx, purchaseContracts.ClientRulesEvaluation{ClientID: p.ClientID, ProjectID: project, Category: item.CategoryCode, Family: item.FamilyCode, Configuration: item.ConfigurationCode, Manufacturer: item.ManufacturerID, Today: time.Now().In(time.FixedZone("Asia/Kolkata", 19800)).Format("2006-01-02")})
	if e != nil {
		return e
	}
	// Only applicable configured instructions restrict makes. An absent policy
	// does not create a review prerequisite; release rechecks restrictions/holds.
	if v.Status == "blocked" {
		return domain.Invalid("item_id", "Client manufacturer restriction: "+strings.Join(v.Reasons, "; "))
	}
	return nil
}
func (c *itemContext) Committed(ctx context.Context, project, id string) (string, error) {
	if c.p.Commitments == nil {
		return "", domain.Invalid("quantity", "Released quantity source is unavailable")
	}
	return c.p.Commitments.Committed(ctx, project, id)
}

type repository struct {
	q *db.Queries
	j events.Writer
}

func view(v db.GetDemandRow) (domain.Record, error) {
	out := domain.Record{ID: v.ID.String(), ProjectID: v.ProjectID.String(), ItemID: v.ItemID.String(), EngineeringKey: v.EngineeringKey, Quantity: v.Quantity, RequiredDate: v.RequiredDate, Active: v.IsActive, Version: v.Version, CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time, CreatedBy: actor(v.CreatedByID), UpdatedBy: actor(v.UpdatedByID)}
	if e := json.Unmarshal(v.ItemSnapshot, &out.Snapshot); e != nil {
		return out, e
	}
	if e := json.Unmarshal(v.Allocations, &out.Allocations); e != nil {
		return out, e
	}
	e := json.Unmarshal(v.Alternatives, &out.Alternatives)
	return out, e
}
func (r *repository) List(ctx context.Context, project string) ([]domain.Record, error) {
	rows, e := r.q.ListDemands(ctx, uuid(project))
	if e != nil {
		return nil, e
	}
	out := []domain.Record{}
	for _, row := range rows {
		v, e := view(db.GetDemandRow(row))
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}
func (r *repository) Get(ctx context.Context, project, id string) (*domain.Record, error) {
	if id == "" {
		return nil, nil
	}
	row, e := r.q.GetDemand(ctx, db.GetDemandParams{ProjectID: uuid(project), ID: uuid(id)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	v, e := view(row)
	return &v, e
}
func (r *repository) Save(ctx context.Context, v domain.Record, actor string) (domain.Record, error) {
	snapshot, e := json.Marshal(v.Snapshot)
	if e != nil {
		return v, e
	}
	allocations, e := json.Marshal(v.Allocations)
	if e != nil {
		return v, e
	}
	alternatives, e := json.Marshal(v.Alternatives)
	if e != nil {
		return v, e
	}
	var row db.GetDemandRow
	if v.Version == 1 {
		x, err := r.q.InsertDemand(ctx, db.InsertDemandParams{ID: uuid(v.ID), ProjectID: uuid(v.ProjectID), ItemID: uuid(v.ItemID), EngineeringKey: v.EngineeringKey, ItemSnapshot: snapshot, Quantity: v.Quantity, RequiredDate: v.RequiredDate, Allocations: allocations, Alternatives: alternatives, IsActive: v.Active, ActorID: uuid(actor)})
		row, e = db.GetDemandRow(x), err
	} else {
		x, err := r.q.UpdateDemand(ctx, db.UpdateDemandParams{ID: uuid(v.ID), ProjectID: uuid(v.ProjectID), Quantity: v.Quantity, RequiredDate: v.RequiredDate, Allocations: allocations, Alternatives: alternatives, IsActive: v.Active, ActorID: uuid(actor), ExpectedVersion: v.Version - 1})
		row, e = db.GetDemandRow(x), err
	}
	if errors.Is(e, pgx.ErrNoRows) {
		return v, domain.Stale
	}
	var pg *pgconn.PgError
	if errors.As(e, &pg) && pg.Code == "23505" {
		return v, domain.Invalid("item_id", "This SKU or engineering configuration already exists in the project")
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
	if e = r.q.InsertDemandEvent(ctx, db.InsertDemandEventParams{ID: uuid(v.ID), SubjectID: uuid(v.SubjectID), Action: v.Action, Reason: v.Reason, BeforeSnapshot: before, AfterSnapshot: after, ActorID: uuid(v.ActorID)}); e != nil {
		return e
	}
	return r.j.Record(ctx, events.Event{ID: v.ID, Module: "project_items", EntityType: "project_material_requirement", EntityID: v.SubjectID, EntityVersion: int64(v.After.Version), Action: v.Action, ActorID: v.ActorID, ActorKind: "user", CorrelationID: v.ID, ScopeKind: "project", ScopeID: v.After.ProjectID, Visibility: "restricted", OccurredAt: v.CreatedAt}, nil)
}
func (r *repository) History(ctx context.Context, project, id string) ([]domain.Event, error) {
	rows, e := r.q.DemandHistory(ctx, db.DemandHistoryParams{ProjectID: uuid(project), ID: uuid(id)})
	if e != nil {
		return nil, e
	}
	out := []domain.Event{}
	for _, row := range rows {
		v := domain.Event{ID: row.ID.String(), SubjectID: row.SubjectID.String(), Action: row.Action, Reason: row.Reason, CreatedAt: row.CreatedAt.Time, ActorID: row.CreatedByID.String()}
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

type DemandPort struct {
	connection database.Connection
	repo       *repository
	writer     *application.Writer
}

func NewDemandPort(c database.Connection, p Providers, j events.Writer) *DemandPort {
	r := &repository{db.New(c), j}
	return &DemandPort{c, r, application.NewWriter(r, &itemContext{p}, Values{})}
}
func (p *DemandPort) List(ctx context.Context, project string) ([]domain.Record, error) {
	return p.repo.List(ctx, project)
}
func (p *DemandPort) Get(ctx context.Context, project, id string) (*domain.Record, error) {
	return p.repo.Get(ctx, project, id)
}
func (p *DemandPort) Save(ctx context.Context, project, id string, input domain.Save, actor string) (domain.Record, error) {
	if !database.IsTransactionConnection(ctx, p.connection) {
		return domain.Record{}, domain.Invalid("", "Material publication must use its consumer's shared transaction")
	}
	return p.writer.Save(ctx, project, id, input, actor)
}

func (c *itemContext) Catalogue(ctx context.Context, project string) ([]domain.CatalogueChoice, error) {
	rows, e := c.p.Items.List(ctx)
	if e != nil {
		return nil, e
	}
	out := []domain.CatalogueChoice{}
	for _, item := range rows {
		if !item.Active || item.Status != "approved" {
			continue
		}
		snapshot, e := c.Item(ctx, item.ID, true)
		if e != nil {
			var rejected *domain.Error
			if errors.As(e, &rejected) && (rejected.Status == 422 || rejected.Status == 404) {
				continue
			}
			return nil, e
		}
		if e = c.Policy(ctx, project, snapshot); e != nil {
			var rejected *domain.Error
			if errors.As(e, &rejected) && rejected.Status == 422 {
				continue
			}
			return nil, e
		}
		out = append(out, domain.CatalogueChoice{ID: item.ID, SKU: item.SKU, Description: item.Description, EngineeringKey: item.EngineeringKey, UOM: snapshot.UOM, Manufacturer: snapshot.Manufacturer, Category: snapshot.CategoryCode, Configuration: snapshot.FamilyCode, Model: item.Model})
	}
	return out, nil
}
