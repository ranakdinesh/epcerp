package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/dtos"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/sqlc"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
)

type profileRepository struct{ *Store }

func (r *profileRepository) Get(ctx context.Context, id string) (*domain.Profile, error) {
	v, e := r.queries.GetProfile(ctx, dtos.UUID(id))
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	p := dtos.Profile(v)
	return &p, e
}

func (r *profileRepository) Save(ctx context.Context, id string, name *string, key, hash string, replace bool) (domain.Profile, error) {
	v, e := r.queries.SaveProfile(ctx, dtos.ProfileSave(id, name, key, hash, replace))
	return dtos.Profile(v), e
}

func (r *profileRepository) SaveContact(ctx context.Context, id string, c domain.PersonalContact, eventID, reason string) error {
	if e := r.queries.SaveEmployeeContact(ctx, sqlc.SaveEmployeeContactParams{ID: dtos.UUID(id), FirstName: c.FirstName, LastName: c.LastName, Mobile: c.Mobile, UpdatedByID: dtos.UUID(id)}); e != nil {
		return e
	}
	// Invalidate open administration forms and retain their organisation history.
	if e := r.queries.AdvancePersonalOrganisationVersion(ctx, dtos.UUID(id)); e != nil {
		return e
	}
	return r.queries.RecordOrganisationEmployeeChange(ctx, sqlc.RecordOrganisationEmployeeChangeParams{EventID: dtos.UUID(eventID), EntityID: dtos.UUID(id), ActorID: dtos.UUID(id), Reason: reason})
}
func (r *profileRepository) Record(ctx context.Context, c domain.ProfileChange) error {
	if c.After != nil {
		before, e := dtos.ContactSnapshot(c.Before)
		if e != nil {
			return e
		}
		after, e := dtos.ContactSnapshot(c.After)
		if e != nil {
			return e
		}
		if e = r.queries.RecordPersonalContactChange(ctx, sqlc.RecordPersonalContactChangeParams{ID: dtos.UUID(c.ID), UserID: dtos.UUID(c.UserID), Version: c.Version, Reason: c.Reason, BeforeSnapshot: before, AfterSnapshot: after, ActorID: dtos.UUID(c.UserID)}); e != nil {
			return e
		}
	}

	return r.events.Record(ctx, events.Event{ID: c.ID, CorrelationID: c.ID, Module: "auth", EntityType: "user_profile", EntityID: c.UserID, EntityVersion: c.Version, Action: c.Action, ActorID: c.UserID, ActorKind: "user", ScopeKind: "platform", Visibility: "restricted", OccurredAt: c.At}, nil)
}
