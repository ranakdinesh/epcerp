package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	projects "github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"
	dtos "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	application "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type IndentGroupsProviders struct {
	Scope     contracts.WorkspaceAccessGate
	BO        contracts.BOReader
	Suppliers masterContracts.VendorPurchaseDirectory
	Projects  projects.ContextReader
	Clients   masterContracts.ClientDirectory
}
type IndentGroupsFactory func(database.Connection) IndentGroupsProviders
type IndentGroupsUnitOfWork = unitOfWork[ports.IndentGroupsRepository, ports.IndentGroupsContext]

func NewIndentGroupsUnitOfWork(b database.Beginner, p IndentGroupsFactory, j func(database.Connection) events.Writer) *IndentGroupsUnitOfWork {
	return newUnitOfWork(b, func(connection database.Connection) (ports.IndentGroupsRepository, ports.IndentGroupsContext) {
		return &indentGroupsRepository{NewStore(connection), j(connection)}, &indentGroupsGroupContext{p(connection)}
	})
}

type indentGroupsGroupContext struct{ p IndentGroupsProviders }

func (c *indentGroupsGroupContext) Scope(ctx context.Context, a domain.Principal, project, permission string, write bool) (bool, error) {
	v, e := c.p.Scope.Check(ctx, contracts.WorkspacePrincipal{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, project, permission, write)
	if errors.Is(e, contracts.WorkspaceErrUnauthorized) {
		return false, domain.IndentGroupsUnauthorized
	}
	if errors.Is(e, contracts.WorkspaceErrForbidden) {
		return false, domain.IndentGroupsForbidden
	}
	if errors.Is(e, contracts.WorkspaceErrMissing) {
		return false, domain.IndentGroupsMissing
	}
	var invalid *contracts.WorkspaceInvalid
	if errors.As(e, &invalid) {
		return false, domain.IndentGroupsInvalid(invalid.Field, invalid.Message)
	}
	return v.CanWrite, e
}
func (c *indentGroupsGroupContext) Source(ctx context.Context, project, id string) (domain.IndentGroupsSource, error) {
	b, e := c.p.BO.Get(ctx, project, id)
	if e != nil {
		return domain.IndentGroupsSource{}, e
	}
	if b == nil || !b.Active {
		return domain.IndentGroupsSource{}, domain.IndentGroupsMissing
	}
	p, e := c.p.Projects.Get(ctx, project, false)
	if e != nil {
		return domain.IndentGroupsSource{}, e
	}
	if p == nil {
		return domain.IndentGroupsSource{}, domain.IndentGroupsMissing
	}
	client, e := c.p.Clients.Get(ctx, p.ClientID)
	if e != nil {
		return domain.IndentGroupsSource{}, e
	}
	source := domain.IndentGroupsSource{Version: b.Version, State: b.State, Lines: []domain.IndentGroupsLine{}, Available: p.Active && p.PurchaseEnabled && p.Lifecycle != "closed" && p.Lifecycle != "on_hold" && client != nil && client.Active}
	for _, l := range b.Lines {
		source.Lines = append(source.Lines, domain.IndentGroupsLine{ID: l.ID, ItemID: l.ItemID, Quantity: l.Quantity, RequiredDate: l.RequiredDate, Reference: l.Reference, Notes: l.Notes, DemandID: l.DemandID, Snapshot: domain.IndentGroupsItem{ID: l.Snapshot.ID, SKU: l.Snapshot.SKU, Description: l.Snapshot.Description, EngineeringKey: l.Snapshot.EngineeringKey, UOM: l.Snapshot.UOM, Manufacturer: l.Snapshot.Manufacturer}})
	}
	return source, nil
}
func (c *indentGroupsGroupContext) Vendors(ctx context.Context) ([]domain.IndentGroupsVendor, error) {
	rows, e := c.p.Suppliers.Candidates(ctx)
	if e != nil {
		return nil, e
	}
	out := []domain.IndentGroupsVendor{}
	for _, r := range rows {
		out = append(out, domain.IndentGroupsVendor{ID: r.ID, Name: r.Name, Code: r.Code, Active: r.Active, Version: r.Version, Data: r.Data})
	}
	return out, nil
}

type indentGroupsRepository struct {
	q *sqlc.Queries
	j events.Writer
}

func (r *indentGroupsRepository) Get(ctx context.Context, project, bo string) (*domain.IndentGroupsRecord, error) {
	row, e := r.q.IndentGroupsGetGroups(ctx, sqlc.IndentGroupsGetGroupsParams{ProjectID: dtos.UUID(project), BoID: dtos.UUID(bo)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	v, e := dtos.IndentGroupsView(row)
	return &v, e
}
func (r *indentGroupsRepository) Save(ctx context.Context, v domain.IndentGroupsRecord, actorID string) (domain.IndentGroupsRecord, error) {
	data, e := json.Marshal(v)
	if e != nil {
		return v, e
	}
	var row sqlc.PurchaseIndentGroupSet
	if v.Version == 1 {
		row, e = r.q.IndentGroupsInsertGroups(ctx, sqlc.IndentGroupsInsertGroupsParams{BoID: dtos.UUID(v.BOID), ProjectID: dtos.UUID(v.ProjectID), Data: data, ActorID: dtos.UUID(actorID)})
	} else {
		row, e = r.q.IndentGroupsUpdateGroups(ctx, sqlc.IndentGroupsUpdateGroupsParams{BoID: dtos.UUID(v.BOID), ProjectID: dtos.UUID(v.ProjectID), Data: data, IsActive: v.Active, ActorID: dtos.UUID(actorID), ExpectedVersion: v.Version - 1})
	}
	if errors.Is(e, pgx.ErrNoRows) {
		return v, domain.IndentGroupsStale
	}
	if e != nil {
		return v, e
	}
	ids := []pgtype.UUID{}
	for _, g := range v.Groups {
		snapshot, e := json.Marshal(g)
		if e != nil {
			return v, e
		}
		_, e = r.q.IndentGroupsSaveGroupEntry(ctx, sqlc.IndentGroupsSaveGroupEntryParams{ID: dtos.UUID(g.ID), BoID: dtos.UUID(v.BOID), Name: g.Name, ScopeRevision: g.ScopeRevision, Data: snapshot, ActorID: dtos.UUID(actorID)})
		if errors.Is(e, pgx.ErrNoRows) {
			return v, domain.IndentGroupsStale
		}
		if e != nil {
			return v, e
		}
		ids = append(ids, dtos.UUID(g.ID))
	}
	if e = r.q.IndentGroupsDeactivateGroupEntries(ctx, sqlc.IndentGroupsDeactivateGroupEntriesParams{BoID: dtos.UUID(v.BOID), ActorID: dtos.UUID(actorID), GroupIds: ids}); e != nil {
		return v, e
	}
	return dtos.IndentGroupsView(row)
}
func (r *indentGroupsRepository) Event(ctx context.Context, v domain.IndentGroupsEvent) error {
	before, e := json.Marshal(v.Before)
	if e != nil {
		return e
	}
	after, e := json.Marshal(v.After)
	if e != nil {
		return e
	}
	if e = r.q.IndentGroupsInsertGroupEvent(ctx, sqlc.IndentGroupsInsertGroupEventParams{ID: dtos.UUID(v.ID), BoID: dtos.UUID(v.BOID), Action: v.Action, Reason: v.Reason, BeforeSnapshot: before, AfterSnapshot: after, ActorID: dtos.UUID(v.ActorID)}); e != nil {
		return e
	}
	return r.j.Record(ctx, events.Event{ID: v.ID, Module: "purchase_indent_groups", EntityType: "purchase_indent_group_set", EntityID: v.BOID, EntityVersion: int64(v.After.Version), Action: v.Action, ActorID: v.ActorID, ActorKind: "user", CorrelationID: v.ID, ScopeKind: "project", ScopeID: v.After.ProjectID, Visibility: "restricted", OccurredAt: v.CreatedAt}, nil)
}
func (r *indentGroupsRepository) History(ctx context.Context, project, bo string) ([]domain.IndentGroupsEvent, error) {
	rows, e := r.q.IndentGroupsGroupHistory(ctx, sqlc.IndentGroupsGroupHistoryParams{ProjectID: dtos.UUID(project), BoID: dtos.UUID(bo)})
	if e != nil {
		return nil, e
	}
	out := []domain.IndentGroupsEvent{}
	for _, r := range rows {
		v := domain.IndentGroupsEvent{ID: r.ID.String(), BOID: r.BoID.String(), Action: r.Action, Reason: r.Reason, CreatedAt: r.CreatedAt.Time, ActorID: r.CreatedByID.String()}
		if e = json.Unmarshal(r.BeforeSnapshot, &v.Before); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(r.AfterSnapshot, &v.After); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}

type IndentGroupsReader struct{ r *indentGroupsRepository }

func NewIndentGroupsReader(c database.Connection) *IndentGroupsReader {
	return &IndentGroupsReader{&indentGroupsRepository{q: NewStore(c)}}
}
func (r *IndentGroupsReader) Get(ctx context.Context, project, bo string) (*domain.IndentGroupsRecord, error) {
	return r.r.Get(ctx, project, bo)
}

type indentGroupsBoundUnit struct {
	c database.Connection
	r ports.IndentGroupsRepository
	p ports.IndentGroupsContext
}

type indentGroupsResolvedActor struct{ a domain.Principal }

func (a indentGroupsResolvedActor) Resolve(context.Context, string) (domain.Principal, error) {
	return a.a, nil
}

type IndentGroupsCommander struct {
	c database.Connection
	p IndentGroupsProviders
	j events.Writer
}

func NewIndentGroupsCommander(c database.Connection, p IndentGroupsProviders, j events.Writer) *IndentGroupsCommander {
	return &IndentGroupsCommander{c, p, j}
}
func (c *IndentGroupsCommander) Execute(ctx context.Context, a domain.Principal, project, bo string, input domain.IndentGroupsCommand) (domain.IndentGroupsWorkspace, error) {
	if !database.IsTransactionConnection(ctx, c.c) {
		return domain.IndentGroupsWorkspace{}, domain.IndentGroupsInvalid("", "Group publication must use its consumer's shared transaction")
	}
	u := indentGroupsBoundUnit{c.c, &indentGroupsRepository{NewStore(c.c), c.j}, &indentGroupsGroupContext{c.p}}
	return application.NewIndentGroups(u, indentGroupsResolvedActor{a}, Values{}).Execute(ctx, "", project, bo, input)
}

func (u indentGroupsBoundUnit) Within(ctx context.Context, f func(context.Context, ports.IndentGroupsRepository, ports.IndentGroupsContext) error) error {
	if !database.IsTransactionConnection(ctx, u.c) {
		return domain.IndentGroupsInvalid("", "Group publication must use its consumer's shared transaction")
	}
	return f(ctx, u.r, u.p)
}

func (r *indentGroupsRepository) PendingGroups(ctx context.Context, p, b string) ([]string, error) {
	rows, e := r.q.QuotationPendingGroups(ctx, sqlc.QuotationPendingGroupsParams{ProjectID: dtos.UUID(p), BoID: dtos.UUID(b)})
	if e != nil {
		return nil, e
	}
	out := []string{}
	for _, row := range rows {
		out = append(out, row.String())
	}
	return out, nil
}
