package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/dtos"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
)

type accountRepository struct{ *Store }

func (r *accountRepository) RecordAccountChange(ctx context.Context, eventID, userID, action string) error {
	if r.events == nil {
		return errors.New("account event writer required")
	}
	v, e := r.queries.AdvanceAccountVersion(ctx, dtos.UUID(userID))
	if e != nil {
		return e
	}
	return r.events.Record(ctx, events.Event{ID: eventID, CorrelationID: eventID, Module: "auth", EntityType: "account", EntityID: userID, EntityVersion: v.AccountVersion, Action: action, ActorID: userID, ActorKind: "user", ScopeKind: "platform", Visibility: "restricted", OccurredAt: v.UpdatedAt.Time}, nil)
}

func (r *accountRepository) UpdateUser(ctx context.Context, id string, request domain.UserUpdate, actor string) (*domain.User, error) {
	return (&userRepository{r.Store}).UpdateUser(ctx, id, request, actor)
}
