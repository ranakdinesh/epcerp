package repository

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/sqlc"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

func (r *Store) AccessRevoked(ctx context.Context, hash string) (bool, error) {
	return r.queries.AccessRevoked(ctx, hash)
}
func (r *Store) SetActor(ctx context.Context, id string) error {
	return r.queries.SetAuthActor(ctx, id)
}
func (r *sessionRepository) TrackAccess(ctx context.Context, hash, user, family string, expiry int64) error {
	return r.queries.TrackAccess(ctx, sqlc.TrackAccessParams{TokenHash: hash, UserID: dtos.UUID(user), FamilyID: dtos.UUID(family), ExpiresAt: dtos.Stamp(domain.At(expiry))})
}
func (r *sessionRepository) LoginBlocked(ctx context.Context, key string) (bool, error) {
	return r.queries.LoginBlocked(ctx, key)
}
func (r *sessionRepository) FailLogin(ctx context.Context, key string) error {
	return r.queries.FailLogin(ctx, key)
}
func (r *sessionRepository) ClearLoginFailures(ctx context.Context, key string) error {
	return r.queries.ClearLoginFailures(ctx, key)
}
func (r *sessionRepository) Sessions(ctx context.Context, user, hash string) ([]domain.Session, error) {
	rows, e := r.queries.ListSessions(ctx, dtos.UUID(user))
	if e != nil {
		return nil, e
	}
	family, e := r.queries.AccessFamily(ctx, hash)
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return nil, e
	}
	out := []domain.Session{}
	for _, v := range rows {
		out = append(out, domain.Session{ID: v.FamilyID.String(), CreatedAt: v.CreatedAt.Time, ExpiresAt: v.ExpiresAt.Time, Current: v.FamilyID == family})
	}
	return out, nil
}
func (r *sessionRepository) RevokeSession(ctx context.Context, user, family string) error {
	owner, e := r.queries.SessionOwner(ctx, dtos.UUID(family))
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.ErrUserMissing
	}
	if e != nil {
		return e
	}
	if owner.String() != user {
		return domain.ErrForbidden
	}
	if e = r.Lock(ctx, "auth.refresh.family:"+family); e != nil {
		return e
	}
	return r.RevokeFamily(ctx, family)
}
func (r *sessionRepository) RevokeAllSessions(ctx context.Context, user string) error {
	if _, e := r.User(ctx, user, true); e != nil {
		return e
	}
	return r.RevokeUserSessions(ctx, user)
}

func (r *roleRepository) AdministrationHistory(ctx context.Context, entity string) ([]domain.AdministrationChange, error) {
	rows, e := r.queries.ListAdministrationChanges(ctx, dtos.UUID(entity))
	if e != nil {
		return nil, e
	}
	out := []domain.AdministrationChange{}
	for _, v := range rows {
		out = append(out, domain.AdministrationChange{ID: v.ID, EntityType: v.EntityType, EntityID: v.EntityID.String(), Action: v.Action, ActorID: auditActor(v.ActorID.String(), v.ActorID.Valid), Before: v.BeforeSnapshot, After: v.AfterSnapshot, OccurredAt: v.OccurredAt.Time})
	}
	return out, nil
}

func auditActor(id string, valid bool) string {
	if !valid {
		return ""
	}
	return id
}
