package repository

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/sqlc"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/ports"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
)

type offboardingRepository struct{ *organisationRepository }

func NewOffboardingUnitOfWork(b database.Beginner, j func(database.Connection) events.Writer, w func(database.Connection) map[string]contracts.WorkSource) ports.UnitOfWork[ports.OffboardingRepository] {
	return &unitOfWork[ports.OffboardingRepository]{manager: database.NewManager(b), events: j, work: w, bind: func(s *Store) ports.OffboardingRepository { return &offboardingRepository{&organisationRepository{s}} }}
}
func (r *offboardingRepository) Cases(ctx context.Context, actor string, admin bool) ([]domain.OffboardingCase, error) {
	rows, e := r.queries.OffboardingCases(ctx, sqlc.OffboardingCasesParams{ActorID: dtos.UUID(actor), Administrator: admin})
	if e != nil {
		return nil, e
	}
	out := []domain.OffboardingCase{}
	for _, v := range rows {
		c, e := dtos.Offboarding(v)
		if e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, nil
}
func (r *offboardingRepository) Case(ctx context.Context, id string) (*domain.OffboardingCase, error) {
	row, e := r.queries.OffboardingCase(ctx, dtos.UUID(id))
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	c, e := dtos.Offboarding(row)
	return &c, e
}
func (r *offboardingRepository) SaveCase(ctx context.Context, c domain.OffboardingCase, actor string) (domain.OffboardingCase, error) {
	items, e := json.Marshal(c.Items)
	if e != nil {
		return c, e
	}
	stamp := pgtype.Timestamptz{}
	if c.ApprovedAt != nil {
		stamp = dtos.Stamp(*c.ApprovedAt)
	}
	row, e := r.queries.SaveOffboardingCase(ctx, sqlc.SaveOffboardingCaseParams{ID: dtos.UUID(c.ID), UserID: dtos.UUID(c.UserID), ManagerID: dtos.UUID(c.ManagerID), ProfileVersion: c.ProfileVersion, State: c.State, Reason: c.Reason, Items: items, ApprovalByID: dtos.OptionalUUID(c.ApprovalByID), ApprovedAt: stamp, IsActive: c.Open(), ActorID: dtos.UUID(actor), ExpectedVersion: c.Version})
	if errors.Is(e, pgx.ErrNoRows) {
		return c, domain.ErrOffboardingStale
	}
	if unique(e) {
		return c, &domain.Error{Kind: domain.Conflict, Detail: "An open offboarding case already exists for this employee."}
	}
	if e != nil {
		return c, e
	}
	return dtos.Offboarding(row)
}
func (r *offboardingRepository) RecordOffboarding(ctx context.Context, c domain.OffboardingCase, event, actor, action, reason string) error {
	if e := r.queries.RecordOffboardingChange(ctx, sqlc.RecordOffboardingChangeParams{EventID: dtos.UUID(event), CaseID: dtos.UUID(c.ID), ActorID: dtos.UUID(actor), Action: action, Reason: reason}); e != nil {
		return e
	}
	if r.events == nil {
		return errors.New("offboarding event writer required")
	}
	return r.events.Record(ctx, events.Event{ID: event, CorrelationID: c.ID, Module: "auth", EntityType: "offboarding", EntityID: c.ID, EntityVersion: c.Version, Action: action, ActorID: actor, ActorKind: "user", ScopeKind: "platform", Visibility: "restricted", OccurredAt: c.UpdatedAt}, nil)
}
func (r *offboardingRepository) ActiveSuperusers(ctx context.Context) (int64, error) {
	return r.queries.CountActiveSuperusers(ctx)
}
func (r *offboardingRepository) Deactivate(ctx context.Context, id, actor string) error {
	return (&userRepository{r.Store}).Deactivate(ctx, id, actor)
}

func (r *offboardingRepository) OffboardingHistory(ctx context.Context, id string) ([]domain.OffboardingChange, error) {
	rows, e := r.queries.OffboardingHistory(ctx, dtos.UUID(id))
	out := []domain.OffboardingChange{}
	for _, v := range rows {
		out = append(out, domain.OffboardingChange{ID: v.ID.String(), Action: v.Action, Reason: v.Reason, ActorID: v.CreatedByID.String(), ActorName: v.ActorName, Version: v.Version, At: v.CreatedAt.Time})
	}
	return out, e
}
