package repository

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/dtos"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/sqlc"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
)

func (r *accountRepository) UsernameAvailable(ctx context.Context, id, name string) (bool, error) {
	v, e := r.queries.UsernameAvailable(ctx, sqlc.UsernameAvailableParams{Username: name, ID: dtos.UUID(id)})
	return v.Bool, e
}
func (r *accountRepository) SaveUsername(ctx context.Context, eventID, id, previous, name string) error {
	if e := r.queries.SaveUsername(ctx, sqlc.SaveUsernameParams{UpdatedByID: dtos.UUID(id), Username: name}); e != nil {
		if unique(e) {
			return domain.UsernameTaken(name)
		}
		return e
	}
	version, e := r.queries.AdvanceAccountVersion(ctx, dtos.UUID(id))
	if e != nil {
		return e
	}
	if e = r.queries.RecordUsernameChange(ctx, sqlc.RecordUsernameChangeParams{ID: dtos.UUID(eventID), UserID: dtos.UUID(id), Version: version.AccountVersion, PreviousUsername: previous, Username: name}); e != nil {
		return e
	}
	return r.events.Record(ctx, events.Event{ID: eventID, CorrelationID: eventID, Module: "auth", EntityType: "account", EntityID: id, EntityVersion: version.AccountVersion, Action: "username_changed", ActorID: id, ActorKind: "user", ScopeKind: "platform", Visibility: "restricted", OccurredAt: version.UpdatedAt.Time}, nil)
}
