package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	projects "github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"
	dtos "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type ClientRulesProviders struct {
	Clients    masterContracts.ClientDirectory
	Projects   projects.ContextReader
	Categories masterContracts.CategoryEligibilityReader
	Items      masterContracts.CategoryItemReader
	Makes      masterContracts.ManufacturerItemReader
}
type ClientRulesFactory func(database.Connection) ClientRulesProviders

type ClientRulesUnitOfWork = unitOfWork[ports.ClientRulesRepository, ports.ClientRulesContext]

func NewClientRulesUnitOfWork(b database.Beginner, p ClientRulesFactory, j func(database.Connection) events.Writer) *ClientRulesUnitOfWork {
	return newUnitOfWork(b, func(connection database.Connection) (ports.ClientRulesRepository, ports.ClientRulesContext) {
		return &clientRulesRepository{NewStore(connection), j(connection)}, &clientRulesRuleContext{p(connection)}
	})
}

type clientRulesRuleContext struct{ p ClientRulesProviders }

func (c *clientRulesRuleContext) Validate(ctx context.Context, client string, d domain.ClientRulesData) error {
	v, e := c.p.Clients.Get(ctx, client)
	if e != nil {
		return e
	}
	if v == nil || !v.Active {
		return domain.ClientRulesInvalid("client_id", "Select an active client")
	}
	if d.ProjectID != nil {
		p, e := c.p.Projects.Get(ctx, *d.ProjectID, false)
		if e != nil {
			return e
		}
		if p == nil || p.ClientID != client {
			return domain.ClientRulesInvalid("project_id", "Project belongs to another client")
		}
	}
	if d.ScopeCode != "*" {
		choices, e := c.p.Categories.FindEligibility(ctx, []string{d.ScopeCode})
		if e != nil {
			return e
		}
		valid := false
		for _, r := range choices {
			if r.Active && r.Code == d.ScopeCode {
				valid = true
			}
		}
		if !valid {
			items, e := c.p.Items.ItemContexts(ctx)
			if e != nil {
				return e
			}
			for _, item := range items {
				if item.Active && item.FamilyCode == d.ScopeCode {
					valid = true
				}
			}
		}
		if !valid {
			return domain.ClientRulesInvalid("scope_code", "Unknown or inactive category/configuration scope")
		}
	}
	for _, id := range append(append([]string{}, d.Allowed...), d.Excluded...) {
		m, e := c.p.Makes.GetMake(ctx, id)
		if e != nil {
			return e
		}
		if m.ID == "" || !m.Active {
			return domain.ClientRulesInvalid("allowed", "Manufacturer is inactive or unavailable")
		}
	}
	return nil
}

type clientRulesRepository struct {
	q *sqlc.Queries
	j events.Writer
}

func (r *clientRulesRepository) Lock(ctx context.Context) error { return r.q.ClientRulesLockRules(ctx) }
func (r *clientRulesRepository) List(ctx context.Context, client string) ([]domain.ClientRulesRecord, error) {
	rows, e := r.q.ClientRulesListRules(ctx, dtos.UUID(client))
	if e != nil {
		return nil, e
	}
	out := []domain.ClientRulesRecord{}
	for _, row := range rows {
		v, e := dtos.ClientRulesView(row)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}
func (r *clientRulesRepository) Get(ctx context.Context, client, id string) (*domain.ClientRulesRecord, error) {
	row, e := r.q.ClientRulesGetRule(ctx, sqlc.ClientRulesGetRuleParams{ClientID: dtos.UUID(client), ID: dtos.UUID(id)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	v, e := dtos.ClientRulesView(row)
	return &v, e
}
func (r *clientRulesRepository) Save(ctx context.Context, v domain.ClientRulesRecord, actor string) (domain.ClientRulesRecord, error) {
	data, e := json.Marshal(v.Data)
	if e != nil {
		return v, e
	}
	project := ""
	if v.Data.ProjectID != nil {
		project = *v.Data.ProjectID
	}
	var row sqlc.ClientPurchaseRule
	if v.Version == 1 {
		row, e = r.q.ClientRulesInsertRule(ctx, sqlc.ClientRulesInsertRuleParams{ID: dtos.UUID(v.ID), ClientID: dtos.UUID(v.ClientID), ProjectID: dtos.UUID(project), Data: data, IsActive: v.Active, ActorID: dtos.UUID(actor)})
	} else {
		row, e = r.q.ClientRulesUpdateRule(ctx, sqlc.ClientRulesUpdateRuleParams{ID: dtos.UUID(v.ID), ClientID: dtos.UUID(v.ClientID), ProjectID: dtos.UUID(project), Data: data, IsActive: v.Active, ActorID: dtos.UUID(actor), ExpectedVersion: v.Version - 1})
	}
	if errors.Is(e, pgx.ErrNoRows) {
		return v, domain.ClientRulesStale
	}
	if e != nil {
		return v, e
	}
	return dtos.ClientRulesView(row)
}
func (r *clientRulesRepository) Event(ctx context.Context, v domain.ClientRulesEvent) error {
	before, e := json.Marshal(v.Before)
	if e != nil {
		return e
	}
	after, e := json.Marshal(v.After)
	if e != nil {
		return e
	}
	if e = r.q.ClientRulesInsertRuleEvent(ctx, sqlc.ClientRulesInsertRuleEventParams{ID: dtos.UUID(v.ID), SubjectID: dtos.UUID(v.SubjectID), Action: v.Action, Reason: v.Reason, BeforeSnapshot: before, AfterSnapshot: after, ActorID: dtos.UUID(v.ActorID)}); e != nil {
		return e
	}
	scope, id := "platform", ""
	if v.After.Data.ProjectID != nil {
		scope, id = "project", *v.After.Data.ProjectID
	}
	return r.j.Record(ctx, events.Event{ID: v.ID, Module: "client_purchase_rules", EntityType: "client_purchase_rule", EntityID: v.SubjectID, EntityVersion: int64(v.After.Version), Action: v.Action, ActorID: v.ActorID, ActorKind: "user", CorrelationID: v.ID, ScopeKind: scope, ScopeID: id, Visibility: "restricted", OccurredAt: v.CreatedAt}, nil)
}
func (r *clientRulesRepository) History(ctx context.Context, client, id string) ([]domain.ClientRulesEvent, error) {
	rows, e := r.q.ClientRulesRuleHistory(ctx, sqlc.ClientRulesRuleHistoryParams{ClientID: dtos.UUID(client), ID: dtos.UUID(id)})
	if e != nil {
		return nil, e
	}
	out := []domain.ClientRulesEvent{}
	for _, row := range rows {
		v := domain.ClientRulesEvent{ID: row.ID.String(), SubjectID: row.SubjectID.String(), Action: row.Action, Reason: row.Reason, CreatedAt: row.CreatedAt.Time, ActorID: row.CreatedByID.String()}
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

type ClientRulesEvaluator struct{ r *clientRulesRepository }

func NewClientRulesEvaluator(c database.Connection) *ClientRulesEvaluator {
	return &ClientRulesEvaluator{&clientRulesRepository{q: NewStore(c)}}
}
func (e *ClientRulesEvaluator) Evaluate(ctx context.Context, q domain.ClientRulesEvaluation) (domain.ClientRulesVerdict, error) {
	if err := e.r.Lock(ctx); err != nil {
		return domain.ClientRulesVerdict{}, err
	}
	rows, err := e.r.List(ctx, q.ClientID)
	if err != nil {
		return domain.ClientRulesVerdict{}, err
	}
	return domain.ClientRulesEvaluate(rows, q), nil
}
