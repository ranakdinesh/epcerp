package repository

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/sqlc"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

func (r *Store) CancelEmailChanges(ctx context.Context, user string) error {
	return r.queries.CancelEmailChanges(ctx, dtos.UUID(user))
}
func (r *Store) SaveEmailChange(ctx context.Context, c domain.EmailChange) error {
	return r.queries.InsertEmailChange(ctx, sqlc.InsertEmailChangeParams{TokenHash: c.Hash, UserID: dtos.UUID(c.UserID), Email: c.Email, PreviousEmail: c.PreviousEmail, CredentialVersion: c.CredentialVersion, CreatedByID: dtos.UUID(c.ActorID), ExpiresAt: dtos.Stamp(c.ExpiresAt)})
}
func (r *Store) EmailChange(ctx context.Context, hash string) (*domain.EmailChange, error) {
	v, e := r.queries.LockEmailChange(ctx, hash)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	return &domain.EmailChange{Hash: v.TokenHash, UserID: v.UserID.String(), Email: v.Email, PreviousEmail: v.PreviousEmail, CredentialVersion: v.CredentialVersion, ActorID: v.CreatedByID.String(), ExpiresAt: v.ExpiresAt.Time, Consumed: v.ConsumedAt.Valid}, nil
}

func (r *Store) CancelCredentialForUser(ctx context.Context, user string) error {
	return r.queries.CancelCredentialForUser(ctx, dtos.UUID(user))
}
