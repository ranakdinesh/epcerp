package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	demands "github.com/ranakdinesh/epcERP/internal/modules/project_items/contracts"
	projects "github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"
	dtos "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type IndentsProviders struct {
	Scope    contracts.WorkspaceAccessGate
	Items    masterContracts.ItemReader
	Rules    contracts.ClientRulesEvaluator
	Holds    contracts.HoldsEvaluator
	Projects projects.ContextReader
	Clients  masterContracts.ClientDirectory
	Demands  demands.DemandPort
	Users    auth.AssignmentDirectory
}
type IndentsFactory func(database.Connection) IndentsProviders
type IndentsUnitOfWork = unitOfWork[ports.IndentsRepository, ports.IndentsContext]

func NewIndentsUnitOfWork(b database.Beginner, p IndentsFactory, j func(database.Connection) events.Writer) *IndentsUnitOfWork {
	return newUnitOfWork(b, func(connection database.Connection) (ports.IndentsRepository, ports.IndentsContext) {
		return &indentsRepository{
			NewStore(connection), j(connection)}, &indentsIndentContext{p(connection)}
	})
}

type indentsIndentContext struct{ p IndentsProviders }

func (c *indentsIndentContext) Scope(ctx context.Context, a domain.Principal, project, permission string, write bool) error {
	_, e := c.p.Scope.Check(ctx, contracts.WorkspacePrincipal{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, project, permission, write)
	if errors.Is(e, contracts.WorkspaceErrUnauthorized) {
		return domain.IndentsUnauthorized
	}
	if errors.Is(e, contracts.WorkspaceErrForbidden) {
		return domain.IndentsForbidden
	}
	if errors.Is(e, contracts.WorkspaceErrMissing) {
		return domain.IndentsMissing
	}
	var invalid *contracts.WorkspaceInvalid
	if errors.As(e, &invalid) {
		return domain.IndentsInvalid(invalid.Field, invalid.Message)
	}
	return e
}
func (c *indentsIndentContext) Validate(ctx context.Context, project string, d domain.IndentsInput) error {
	if d.DoerID == d.ResolverID {
		return domain.IndentsInvalid("resolver_id", "Procurement doer and problem resolver must be different people")
	}
	for _, id := range []string{d.DoerID, d.ResolverID} {
		v, e := c.p.Users.Get(ctx, id)
		if e != nil {
			return e
		}
		if v == nil || !v.Active {
			return domain.IndentsInvalid("doer_id", "Assign active doer and resolver identities")
		}
	}
	return nil
}
func (c *indentsIndentContext) Line(ctx context.Context, project, id string) (domain.IndentsSource, error) {
	demand, e := c.p.Demands.Get(ctx, project, id)
	if e != nil {
		return domain.IndentsSource{}, e
	}
	if demand == nil {
		return domain.IndentsSource{}, domain.IndentsMissing
	}
	p, e := c.p.Projects.Get(ctx, project, false)
	if e != nil {
		return domain.IndentsSource{}, e
	}
	if p == nil {
		return domain.IndentsSource{}, domain.IndentsMissing
	}
	client, e := c.p.Clients.Get(ctx, p.ClientID)
	if e != nil {
		return domain.IndentsSource{}, e
	}
	active := demand.Active && client != nil && client.Active
	validate := func(id string) error {
		_, err := c.p.Items.Approved(ctx, id)
		if err != nil {
			var invalid *masterContracts.Error
			if errors.As(err, &invalid) {
				active = false
				return nil
			}
			return err
		}
		return nil
	}
	if e = validate(demand.ItemID); e != nil {
		return domain.IndentsSource{}, e
	}
	item, e := c.p.Items.Get(ctx, demand.ItemID)
	if e != nil {
		return domain.IndentsSource{}, e
	}
	config := item.Snapshot.Configuration
	evaluate := func(manufacturer, configuration string) (contracts.ClientRulesVerdict, error) {
		return c.p.Rules.Evaluate(ctx, contracts.ClientRulesEvaluation{ClientID: p.ClientID, ProjectID: project, Category: config.CategoryCode, Family: config.FamilyCode, Configuration: configuration, Manufacturer: manufacturer, Today: time.Now().In(time.FixedZone("Asia/Kolkata", 19800)).Format("2006-01-02")})
	}
	policy, e := evaluate(item.ManufacturerID, config.Code)
	if e != nil {
		return domain.IndentsSource{}, e
	}
	for _, id := range demand.Alternatives {
		if e = validate(id); e != nil {
			return domain.IndentsSource{}, e
		}
		other, e := c.p.Items.Get(ctx, id)
		if e != nil {
			return domain.IndentsSource{}, e
		}
		alt, e := evaluate(other.ManufacturerID, other.Snapshot.Configuration.Code)
		if e != nil {
			return domain.IndentsSource{}, e
		}
		if alt.Status != "eligible" {
			policy.Status = "blocked"
			policy.Reasons = append(policy.Reasons, "An alternative manufacturer is not client-permitted")
		}
	}
	hold, e := c.p.Holds.Evaluate(ctx, project, id, config.CategoryCode)
	if e != nil {
		return domain.IndentsSource{}, e
	}
	source := domain.IndentsSource{Active: active, Quantity: demand.Quantity, DemandVersion: demand.Version, RequiredDate: demand.RequiredDate, ClientID: p.ClientID, Allocations: []domain.IndentsAllocation{}, Policy: domain.IndentsPolicy{Status: policy.Status, Reasons: policy.Reasons, Rules: []domain.IndentsPolicyEvidence{}}, Holds: domain.IndentsHolds{Holds: []domain.IndentsHold{}, HoldMinutes: hold.HoldMinutes, FinishImpact: hold.FinishImpact}}
	for _, rule := range policy.Rules {
		source.Policy.Rules = append(source.Policy.Rules, domain.IndentsPolicyEvidence{ID: rule.ID, Version: rule.Version})
	}
	for _, h := range hold.Holds {
		source.Holds.Holds = append(source.Holds.Holds, domain.IndentsHold{ID: h.ID, Reason: h.Reason, DoerID: h.DoerID, ResolverID: h.ResolverID, StartedAt: h.StartedAt})
	}
	for _, a := range demand.Allocations {
		source.Allocations = append(source.Allocations, domain.IndentsAllocation{Location: a.Location, Quantity: a.Quantity})
	}
	snapshot, e := json.Marshal(demand.Snapshot)
	if e != nil {
		return source, e
	}
	e = json.Unmarshal(snapshot, &source.Item)
	return source, e
}

type indentsRepository struct {
	q *sqlc.Queries
	j events.Writer
}

func (r *indentsRepository) List(ctx context.Context, project string) ([]domain.IndentsRecord, error) {
	rows, e := r.q.IndentsListIndents(ctx, dtos.UUID(project))
	if e != nil {
		return nil, e
	}
	out := []domain.IndentsRecord{}
	for _, row := range rows {
		v, e := dtos.IndentsView(row)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}
func (r *indentsRepository) Get(ctx context.Context, project, id string) (*domain.IndentsRecord, error) {
	row, e := r.q.IndentsGetIndent(ctx, sqlc.IndentsGetIndentParams{ProjectID: dtos.UUID(project), ID: dtos.UUID(id)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	v, e := dtos.IndentsView(row)
	return &v, e
}
func (r *indentsRepository) Save(ctx context.Context, v domain.IndentsRecord, actor string) (domain.IndentsRecord, error) {
	data, e := json.Marshal(v.Data)
	if e != nil {
		return v, e
	}
	var row sqlc.PurchaseIndent
	if v.Version == 1 {
		row, e = r.q.IndentsInsertIndent(ctx, sqlc.IndentsInsertIndentParams{ID: dtos.UUID(v.ID), ProjectID: dtos.UUID(v.ProjectID), DoerID: dtos.UUID(v.Data.DoerID), ResolverID: dtos.UUID(v.Data.ResolverID), Data: data, ActorID: dtos.UUID(actor)})
	} else {
		row, e = r.q.IndentsUpdateIndent(ctx, sqlc.IndentsUpdateIndentParams{ID: dtos.UUID(v.ID), ProjectID: dtos.UUID(v.ProjectID), Data: data, DoerID: dtos.UUID(v.Data.DoerID), ResolverID: dtos.UUID(v.Data.ResolverID), IsActive: v.Active, ActorID: dtos.UUID(actor), ExpectedVersion: v.Version - 1})
	}
	if errors.Is(e, pgx.ErrNoRows) {
		return v, domain.IndentsStale
	}
	if e != nil {
		return v, e
	}
	for _, release := range v.Data.Released {
		snapshot, err := json.Marshal(release)
		if err != nil {
			return v, err
		}
		n, err := r.q.IndentsInsertRelease(ctx, sqlc.IndentsInsertReleaseParams{IndentID: dtos.UUID(v.ID), DemandID: dtos.UUID(release.DemandID), Quantity: release.Quantity, Snapshot: snapshot, ActorID: dtos.UUID(release.ActorID)})
		if err != nil {
			return v, err
		}
		if n == 0 {
			previous, err := r.q.IndentsGetRelease(ctx, sqlc.IndentsGetReleaseParams{IndentID: dtos.UUID(v.ID), DemandID: dtos.UUID(release.DemandID)})
			if err != nil {
				return v, err
			}
			var prior domain.IndentsRelease
			if err = json.Unmarshal(previous, &prior); err != nil {
				return v, err
			}
			canonical, err := json.Marshal(prior)
			if err != nil {
				return v, err
			}
			if !bytes.Equal(canonical, snapshot) {
				return v, domain.IndentsInvalid("released", "Released source evidence cannot be rewritten")
			}
		}
	}
	return dtos.IndentsView(row)
}
func (r *indentsRepository) Event(ctx context.Context, v domain.IndentsEvent) error {
	before, e := json.Marshal(v.Before)
	if e != nil {
		return e
	}
	after, e := json.Marshal(v.After)
	if e != nil {
		return e
	}
	if e = r.q.IndentsInsertIndentEvent(ctx, sqlc.IndentsInsertIndentEventParams{ID: dtos.UUID(v.ID), SubjectID: dtos.UUID(v.SubjectID), Action: v.Action, Reason: v.Reason, BeforeSnapshot: before, AfterSnapshot: after, ActorID: dtos.UUID(v.ActorID)}); e != nil {
		return e
	}
	return r.j.Record(ctx, events.Event{ID: v.ID, Module: "purchase_indents", EntityType: "purchase_indent", EntityID: v.SubjectID, EntityVersion: int64(v.After.Version), Action: v.Action, ActorID: v.ActorID, ActorKind: "user", CorrelationID: v.ID, ScopeKind: "project", ScopeID: v.After.ProjectID, Visibility: "restricted", OccurredAt: v.CreatedAt}, nil)
}
func (r *indentsRepository) History(ctx context.Context, project, id string) ([]domain.IndentsEvent, error) {
	rows, e := r.q.IndentsIndentHistory(ctx, sqlc.IndentsIndentHistoryParams{ProjectID: dtos.UUID(project), ID: dtos.UUID(id)})
	if e != nil {
		return nil, e
	}
	out := []domain.IndentsEvent{}
	for _, row := range rows {
		v := domain.IndentsEvent{ID: row.ID.String(), SubjectID: row.SubjectID.String(), Action: row.Action, Reason: row.Reason, CreatedAt: row.CreatedAt.Time, ActorID: row.CreatedByID.String()}
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

type IndentsReader struct{ r *indentsRepository }

func NewIndentsReader(c database.Connection) *IndentsReader {
	return &IndentsReader{&indentsRepository{q: NewStore(c)}}
}
func (r *IndentsReader) List(ctx context.Context, project string) ([]domain.IndentsRecord, error) {
	return r.r.List(ctx, project)
}
func (r *IndentsReader) Get(ctx context.Context, project, id string) (*domain.IndentsRecord, error) {
	return r.r.Get(ctx, project, id)
}
func (r *IndentsReader) Committed(ctx context.Context, project, demand string) (string, error) {
	return r.r.q.IndentsCommitted(ctx, sqlc.IndentsCommittedParams{ProjectID: dtos.UUID(project), DemandID: dtos.UUID(demand)})
}
