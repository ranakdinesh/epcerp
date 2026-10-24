package repository

import (
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
	dtos "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type HoldsProviders struct {
	Scope      contracts.WorkspaceHoldGate
	Categories masterContracts.CategoryEligibilityReader
	Demands    demands.DemandPort
	Users      auth.AssignmentDirectory
}
type HoldsFactory func(database.Connection) HoldsProviders
type HoldsUnitOfWork = unitOfWork[ports.HoldsRepository, ports.HoldsContext]

func NewHoldsUnitOfWork(b database.Beginner, p HoldsFactory, j func(database.Connection) events.Writer) *HoldsUnitOfWork {
	return newUnitOfWork(b, func(connection database.Connection) (ports.HoldsRepository, ports.HoldsContext) {
		return &holdsRepository{NewStore(connection), j(connection)}, &holdsHoldContext{p(
			connection)}
	})
}

type holdsHoldContext struct{ p HoldsProviders }

func (c *holdsHoldContext) Scope(ctx context.Context, a domain.Principal, project, permission string, write bool) error {
	_, e := c.p.Scope.CheckHold(ctx, contracts.WorkspacePrincipal{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, project, permission, write)
	if errors.Is(e, contracts.WorkspaceErrUnauthorized) {
		return domain.HoldsUnauthorized
	}
	if errors.Is(e, contracts.WorkspaceErrForbidden) {
		return domain.HoldsForbidden
	}
	if errors.Is(e, contracts.WorkspaceErrMissing) {
		return domain.HoldsMissing
	}
	var invalid *contracts.WorkspaceInvalid
	if errors.As(e, &invalid) {
		return domain.HoldsInvalid(invalid.Field, invalid.Message)
	}
	return e
}
func (c *holdsHoldContext) Validate(ctx context.Context, project string, d domain.HoldsCreate) error {
	if d.Scope == "category" {
		choices, e := c.p.Categories.FindEligibility(ctx, []string{d.CategoryCode})
		if e != nil {
			return e
		}
		valid := false
		for _, v := range choices {
			if v.Active && v.Kind == "category" && v.Code == d.CategoryCode {
				valid = true
			}
		}
		if !valid {
			return domain.HoldsInvalid("category_code", "Select an active category")
		}
	}
	for _, id := range d.DemandIDs {
		v, e := c.p.Demands.Get(ctx, project, id)
		if e != nil {
			return e
		}
		if v == nil {
			return domain.HoldsInvalid("demand_ids", "Select requirements from this project")
		}
	}
	// Owner-authorized cross-project task assignment uses active identities, not an implicit project-membership grant.
	for _, id := range []string{d.DoerID, d.ResolverID} {
		v, e := c.p.Users.Get(ctx, id)
		if e != nil {
			return e
		}
		if v == nil || !v.Active {
			return domain.HoldsInvalid("doer_id", "Doer and resolver must be active users")
		}
	}
	return nil
}

type holdsRepository struct {
	q *sqlc.Queries
	j events.Writer
}

func (r *holdsRepository) List(ctx context.Context, project string) ([]domain.HoldsRecord, error) {
	rows, e := r.q.HoldsListHolds(ctx, dtos.UUID(project))
	if e != nil {
		return nil, e
	}
	out := []domain.HoldsRecord{}
	for _, row := range rows {
		v, e := dtos.HoldsView(row)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}
func (r *holdsRepository) Get(ctx context.Context, project, id string) (*domain.HoldsRecord, error) {
	row, e := r.q.HoldsGetHold(ctx, sqlc.HoldsGetHoldParams{ProjectID: dtos.UUID(project), ID: dtos.UUID(id)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	v, e := dtos.HoldsView(row)
	return &v, e
}
func (r *holdsRepository) Save(ctx context.Context, v domain.HoldsRecord, actor string) (domain.HoldsRecord, error) {
	data, e := json.Marshal(v.Data)
	if e != nil {
		return v, e
	}
	var row sqlc.PurchaseHold
	if v.Version == 1 {
		row, e = r.q.HoldsInsertHold(ctx, sqlc.HoldsInsertHoldParams{ID: dtos.UUID(v.ID), ProjectID: dtos.UUID(v.ProjectID), DoerID: dtos.UUID(v.Data.DoerID), ResolverID: dtos.UUID(v.Data.ResolverID), Data: data, ActorID: dtos.UUID(actor)})
	} else {
		row, e = r.q.HoldsUpdateHold(ctx, sqlc.HoldsUpdateHoldParams{ID: dtos.UUID(v.ID), ProjectID: dtos.UUID(v.ProjectID), Data: data, IsActive: v.Active, ActorID: dtos.UUID(actor), ExpectedVersion: v.Version - 1})
	}
	if errors.Is(e, pgx.ErrNoRows) {
		return v, domain.HoldsStale
	}
	if e != nil {
		return v, e
	}
	return dtos.HoldsView(row)
}
func (r *holdsRepository) Event(ctx context.Context, v domain.HoldsEvent) error {
	before, e := json.Marshal(v.Before)
	if e != nil {
		return e
	}
	after, e := json.Marshal(v.After)
	if e != nil {
		return e
	}
	if e = r.q.HoldsInsertHoldEvent(ctx, sqlc.HoldsInsertHoldEventParams{ID: dtos.UUID(v.ID), SubjectID: dtos.UUID(v.SubjectID), Action: v.Action, Reason: v.Reason, BeforeSnapshot: before, AfterSnapshot: after, ActorID: dtos.UUID(v.ActorID)}); e != nil {
		return e
	}
	return r.j.Record(ctx, events.Event{ID: v.ID, Module: "purchase_holds", EntityType: "purchase_hold", EntityID: v.SubjectID, EntityVersion: int64(v.After.Version), Action: v.Action, ActorID: v.ActorID, ActorKind: "user", CorrelationID: v.ID, ScopeKind: "project", ScopeID: v.After.ProjectID, Visibility: "restricted", OccurredAt: v.CreatedAt}, nil)
}
func (r *holdsRepository) History(ctx context.Context, project, id string) ([]domain.HoldsEvent, error) {
	rows, e := r.q.HoldsHoldHistory(ctx, sqlc.HoldsHoldHistoryParams{ProjectID: dtos.UUID(project), ID: dtos.UUID(id)})
	if e != nil {
		return nil, e
	}
	out := []domain.HoldsEvent{}
	for _, row := range rows {
		v := domain.HoldsEvent{ID: row.ID.String(), SubjectID: row.SubjectID.String(), Action: row.Action, Reason: row.Reason, CreatedAt: row.CreatedAt.Time, ActorID: row.CreatedByID.String()}
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

type HoldsEvaluator struct{ r *holdsRepository }

func NewHoldsEvaluator(c database.Connection) *HoldsEvaluator {
	return &HoldsEvaluator{&holdsRepository{q: NewStore(c)}}
}
func (e *HoldsEvaluator) Evaluate(ctx context.Context, project, demand, category string) (domain.HoldsEvaluation, error) {
	rows, err := e.r.List(ctx, project)
	if err != nil {
		return domain.HoldsEvaluation{}, err
	}
	return domain.HoldsEvaluate(rows, demand, category, time.Now().UTC()), nil
}
