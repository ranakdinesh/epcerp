package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
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

type BOProviders struct {
	Scope    contracts.WorkspaceAccessGate
	Items    masterContracts.ItemReader
	Rules    contracts.ClientRulesEvaluator
	Projects projects.ContextReader
	Demands  demands.DemandPort
}
type BOFactory func(database.Connection) BOProviders
type BOUnitOfWork = unitOfWork[ports.BORepository, ports.BOContext]

func NewBOUnitOfWork(b database.Beginner, p BOFactory, j func(database.Connection) events.Writer) *BOUnitOfWork {
	return newUnitOfWork(b, func(connection database.Connection) (ports.BORepository, ports.BOContext) {
		return &bORepository{NewStore(connection), j(connection)}, &bOBoContext{p(connection)}
	})
}

type bOBoContext struct{ p BOProviders }

func (c *bOBoContext) Scope(ctx context.Context, a domain.Principal, project, permission string, write bool) error {
	_, e := c.p.Scope.Check(ctx, contracts.WorkspacePrincipal{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, project, permission, write)
	if errors.Is(e, contracts.WorkspaceErrUnauthorized) {
		return domain.BOUnauthorized
	}
	if errors.Is(e, contracts.WorkspaceErrForbidden) {
		return domain.BOForbidden
	}
	if errors.Is(e, contracts.WorkspaceErrMissing) {
		return domain.BOMissing
	}
	var invalid *contracts.WorkspaceInvalid
	if errors.As(e, &invalid) {
		return &domain.BOError{Status: 422, Field: invalid.Field, Message: invalid.Message}
	}
	return e
}
func (c *bOBoContext) Item(ctx context.Context, project, id string) (domain.BOSnapshot, error) {
	item, e := c.p.Items.Approved(ctx, id)
	if e != nil {
		var invalid *masterContracts.Error
		if errors.As(e, &invalid) {
			status := 500
			switch invalid.Kind {
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
			return domain.BOSnapshot{}, &domain.BOError{Status: status, Field: "item_id", Message: invalid.Detail}
		}
		return domain.BOSnapshot{}, e
	}
	p, e := c.p.Projects.Get(ctx, project, false)
	if e != nil {
		return domain.BOSnapshot{}, e
	}
	if p == nil {
		return domain.BOSnapshot{}, domain.BOMissing
	}
	config := item.Snapshot.Configuration
	v, e := c.p.Rules.Evaluate(ctx, contracts.ClientRulesEvaluation{ClientID: p.ClientID, ProjectID: project, Category: config.CategoryCode, Family: config.FamilyCode, Configuration: config.Code, Manufacturer: item.ManufacturerID, Today: time.Now().In(time.FixedZone("Asia/Kolkata", 19800)).Format("2006-01-02")})
	if e != nil {
		return domain.BOSnapshot{}, e
	}
	if v.Status == "blocked" {
		return domain.BOSnapshot{}, domain.BOInvalid("item_id", "Client manufacturer restriction: "+strings.Join(v.Reasons, "; "))
	}
	return domain.BOSnapshot{ID: item.ID, SKU: item.SKU, Description: item.Description, EngineeringKey: item.EngineeringKey, UOM: config.UOM, Manufacturer: item.Snapshot.Manufacturer.Name}, nil
}
func bOMaterialError(e error) error {
	var invalid *demands.Error
	if errors.As(e, &invalid) {
		return &domain.BOError{Status: 422, Field: invalid.Field, Message: invalid.Message}
	}
	return e
}
func (c *bOBoContext) DemandVersion(ctx context.Context, project string, line domain.BOLine) (int32, error) {
	if line.DemandID == nil || line.Quantity == nil || line.RequiredDate == nil {
		return 0, domain.BOStale
	}
	v, e := c.p.Demands.Get(ctx, project, *line.DemandID)
	if e != nil {
		return 0, bOMaterialError(e)
	}
	if v == nil || !v.Active || line.ItemID == nil || v.ItemID != *line.ItemID || v.RequiredDate != *line.RequiredDate {
		return 0, domain.BOStale
	}
	q, e := domain.BOQuantity(v.Quantity)
	if e != nil {
		return 0, e
	}
	expected, e := domain.BOQuantity(*line.Quantity)
	if e != nil {
		return 0, e
	}
	if q != expected {
		return 0, domain.BOStale
	}
	return v.Version, nil
}
func (c *bOBoContext) Publish(ctx context.Context, project string, line domain.BOLine, actorID, reason string) (string, error) {
	if line.ItemID == nil || line.Quantity == nil || line.RequiredDate == nil {
		return "", domain.BOInvalid("lines", "Complete the item fields before publication")
	}
	input := demands.Save{ItemID: *line.ItemID, Quantity: *line.Quantity, RequiredDate: *line.RequiredDate, Active: true, Version: 1, Reason: reason}
	id := ""
	if line.DemandID != nil {
		id = *line.DemandID
		v, e := c.p.Demands.Get(ctx, project, id)
		if e != nil {
			return "", bOMaterialError(e)
		}
		if v == nil || line.Snapshot.DemandVersion == nil || v.Version != *line.Snapshot.DemandVersion {
			return "", domain.BOStale
		}
		input.Version = v.Version
		input.Allocations = v.Allocations
		input.Alternatives = v.Alternatives
	}
	v, e := c.p.Demands.Save(ctx, project, id, input, actorID)
	return v.ID, bOMaterialError(e)
}

type bORepository struct {
	q *sqlc.Queries
	j events.Writer
}

func bOOptional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (r *bORepository) view(ctx context.Context, v sqlc.PurchaseBoList) (domain.BORecord, error) {
	out := domain.BORecord{ID: v.ID.String(), ProjectID: v.ProjectID.String(), PreparerID: v.PreparerID.String(), Name: v.Name, State: v.State, Active: v.IsActive, Version: v.Version, CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time, CreatedBy: dtos.Actor(v.CreatedByID), UpdatedBy: dtos.Actor(v.UpdatedByID), CompletedBy: dtos.Actor(v.CompletedByID), Lines: []domain.BOLine{}}
	if v.CompletedAt.Valid {
		out.CompletedAt = &v.CompletedAt.Time
	}
	rows, e := r.q.BOBOLines(ctx, v.ID)
	if e != nil {
		return out, e
	}
	for _, row := range rows {
		line := domain.BOLine{ID: row.ID.String(), ItemID: dtos.Actor(row.ItemID), Quantity: bOOptional(row.Quantity), RequiredDate: bOOptional(row.RequiredDate), Reference: row.Reference, Notes: row.Notes, DemandID: dtos.Actor(row.DemandID)}
		if e = json.Unmarshal(row.ItemSnapshot, &line.Snapshot); e != nil {
			return out, e
		}
		out.Lines = append(out.Lines, line)
	}
	return out, nil
}
func (r *bORepository) List(ctx context.Context, project string) ([]domain.BORecord, error) {
	rows, e := r.q.BOListBO(ctx, dtos.UUID(project))
	if e != nil {
		return nil, e
	}
	out := []domain.BORecord{}
	for _, row := range rows {
		v, e := r.view(ctx, row)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}
func (r *bORepository) Get(ctx context.Context, project, id string) (*domain.BORecord, error) {
	row, e := r.q.BOGetBO(ctx, sqlc.BOGetBOParams{ProjectID: dtos.UUID(project), ID: dtos.UUID(id)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	out, e := r.view(ctx, row)
	return &out, e
}
func (r *bORepository) Save(ctx context.Context, v domain.BORecord, actorID string) (domain.BORecord, error) {
	var row sqlc.PurchaseBoList
	var e error
	stamp := pgtype.Timestamptz{}
	if v.CompletedAt != nil {
		stamp = pgtype.Timestamptz{Time: *v.CompletedAt, Valid: true}
	}
	if v.Version == 1 {
		row, e = r.q.BOInsertBO(ctx, sqlc.BOInsertBOParams{ID: dtos.UUID(v.ID), ProjectID: dtos.UUID(v.ProjectID), PreparerID: dtos.UUID(v.PreparerID), Name: v.Name, State: v.State, CompletedAt: stamp, CompletedByID: dtos.BONullableID(v.CompletedBy), ActorID: dtos.UUID(actorID)})
	} else {
		row, e = r.q.BOUpdateBO(ctx, sqlc.BOUpdateBOParams{ID: dtos.UUID(v.ID), ProjectID: dtos.UUID(v.ProjectID), Name: v.Name, State: v.State, CompletedAt: stamp, CompletedByID: dtos.BONullableID(v.CompletedBy), ActorID: dtos.UUID(actorID), ExpectedVersion: v.Version - 1})
	}
	if errors.Is(e, pgx.ErrNoRows) {
		return v, domain.BOStale
	}
	if e != nil {
		return v, e
	}
	ids := []pgtype.UUID{}
	for i, line := range v.Lines {
		snapshot, e := json.Marshal(line.Snapshot)
		if e != nil {
			return v, e
		}
		_, e = r.q.BOSaveBOLine(ctx, sqlc.BOSaveBOLineParams{ID: dtos.UUID(line.ID), BoID: dtos.UUID(v.ID), ItemID: dtos.BONullableID(line.ItemID), Quantity: dtos.BONullableText(line.Quantity), RequiredDate: dtos.BONullableText(line.RequiredDate), Reference: line.Reference, Notes: line.Notes, ItemSnapshot: snapshot, DemandID: dtos.BONullableID(line.DemandID), SortOrder: int32(i), ActorID: dtos.UUID(actorID)})
		if errors.Is(e, pgx.ErrNoRows) {
			return v, domain.BOStale
		}
		if e != nil {
			return v, e
		}
		ids = append(ids, dtos.UUID(line.ID))
	}
	if e = r.q.BODeactivateMissingLines(ctx, sqlc.BODeactivateMissingLinesParams{BoID: dtos.UUID(v.ID), ActorID: dtos.UUID(actorID), LineIds: ids}); e != nil {
		return v, e
	}
	return r.view(ctx, row)
}
func (r *bORepository) Event(ctx context.Context, v domain.BOEvent) error {
	before, e := json.Marshal(v.Before)
	if e != nil {
		return e
	}
	after, e := json.Marshal(v.After)
	if e != nil {
		return e
	}
	if e = r.q.BOInsertBOEvent(ctx, sqlc.BOInsertBOEventParams{ID: dtos.UUID(v.ID), SubjectID: dtos.UUID(v.SubjectID), Action: v.Action, Reason: v.Reason, BeforeSnapshot: before, AfterSnapshot: after, ActorID: dtos.UUID(v.ActorID)}); e != nil {
		return e
	}
	return r.j.Record(ctx, events.Event{ID: v.ID, Module: "purchase_bo", EntityType: "purchase_bo_list", EntityID: v.SubjectID, EntityVersion: int64(v.After.Version), Action: v.Action, ActorID: v.ActorID, ActorKind: "user", CorrelationID: v.ID, ScopeKind: "project", ScopeID: v.After.ProjectID, Visibility: "restricted", OccurredAt: v.CreatedAt}, nil)
}
func (r *bORepository) History(ctx context.Context, project, id string) ([]domain.BOEvent, error) {
	rows, e := r.q.BOBOHistory(ctx, sqlc.BOBOHistoryParams{ProjectID: dtos.UUID(project), ID: dtos.UUID(id)})
	if e != nil {
		return nil, e
	}
	out := []domain.BOEvent{}
	for _, row := range rows {
		v := domain.BOEvent{ID: row.ID.String(), SubjectID: row.SubjectID.String(), Action: row.Action, Reason: row.Reason, CreatedAt: row.CreatedAt.Time, ActorID: row.CreatedByID.String()}
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

// Reader exposes immutable BO source evidence to an already-authorized consumer.
type BOReader struct{ r *bORepository }

func NewBOReader(c database.Connection) *BOReader {
	return &BOReader{&bORepository{q: NewStore(c)}}
}
func (r *BOReader) List(ctx context.Context, project string) ([]domain.BORecord, error) {
	return r.r.List(ctx, project)
}
func (r *BOReader) Get(ctx context.Context, project, id string) (*domain.BORecord, error) {
	return r.r.Get(ctx, project, id)
}

func (c *bOBoContext) Project(ctx context.Context, id string) (domain.BOProject, error) {
	p, e := c.p.Projects.Get(ctx, id, true)
	if e != nil {
		return domain.BOProject{}, e
	}
	if p == nil {
		return domain.BOProject{}, domain.BOMissing
	}
	return domain.BOProject{Number: p.Number, Name: p.Name}, nil
}
