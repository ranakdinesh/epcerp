package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/sqlc"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

type sessionRepository struct{ *Store }

func (r *sessionRepository) FindUser(ctx context.Context, identifier string) (*domain.User, error) {
	rows, err := r.queries.FindUser(ctx, identifier)
	if err != nil {
		return nil, err
	}
	// Never select an arbitrary identity if a legacy username collides with email.
	if len(rows) != 1 {
		return nil, nil
	}
	return r.user(ctx, rows[0])
}

func (r *sessionRepository) CreateRefresh(ctx context.Context, refresh domain.Refresh) error {
	return r.queries.InsertRefresh(ctx, sqlc.InsertRefreshParams{ID: dtos.UUID(refresh.ID), UserID: dtos.UUID(refresh.UserID), TokenHash: refresh.Hash, FamilyID: dtos.UUID(refresh.FamilyID), ExpiresAt: dtos.Stamp(refresh.ExpiresAt)})
}

func (r *sessionRepository) Refresh(ctx context.Context, hash string, lock bool) (*domain.Refresh, error) {
	var row sqlc.AuthRefreshToken
	var err error
	if lock {
		row, err = r.queries.LockRefresh(ctx, hash)
	} else {
		row, err = r.queries.GetRefresh(ctx, hash)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return dtos.Refresh(row), nil
}

func (r *sessionRepository) RevokeRefresh(ctx context.Context, hash string) error {
	return r.queries.RevokeRefresh(ctx, hash)
}

func (r *sessionRepository) RevokeFamily(ctx context.Context, family string) error {
	return r.queries.RevokeFamily(ctx, dtos.UUID(family))
}

func (r *sessionRepository) RecordActivity(ctx context.Context, userID, page string) error {
	return r.queries.RecordActivity(ctx, sqlc.RecordActivityParams{UserID: dtos.UUID(userID), CurrentPage: page})
}
