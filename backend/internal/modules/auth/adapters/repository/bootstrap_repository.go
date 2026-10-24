package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/sqlc"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

type bootstrapRepository struct{ *Store }

func (r *bootstrapRepository) ReconcilePermission(ctx context.Context, permissionID string, permission domain.Permission) error {
	return r.queries.ReconcilePermission(ctx, sqlc.ReconcilePermissionParams{ID: dtos.UUID(permissionID), Code: permission.Code, Description: permission.Description})
}

func (r *bootstrapRepository) EnsureBootstrapRole(ctx context.Context, roleID string) (string, error) {
	value, err := r.queries.EnsureBootstrapRole(ctx, dtos.UUID(roleID))
	return value.String(), err
}

func (r *bootstrapRepository) BootstrapUser(ctx context.Context, userID, username, email, hash string) (string, bool, error) {
	bound, err := r.queries.BoundBootstrapUser(ctx)
	if err == nil {
		if !bound.IsSuperuser {
			if err = r.requireBootstrapSlot(ctx); err != nil {
				return "", false, err
			}
		}
		return bound.ID.String(), false, r.queries.RepairBootstrapUser(ctx, bound.ID)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", false, err
	}
	row, err := r.queries.FindBootstrapUser(ctx, username)
	if err == nil {
		if !row.IsSuperuser {
			if err = r.requireBootstrapSlot(ctx); err != nil {
				return "", false, err
			}
		}
		if err = r.queries.RepairBootstrapUser(ctx, row.ID); err != nil {
			return "", false, err
		}
		return row.ID.String(), false, r.queries.BindBootstrapUser(ctx, row.ID)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", false, err
	}
	if err = r.requireBootstrapSlot(ctx); err != nil {
		return "", false, err
	}
	value, err := r.queries.InsertBootstrapUser(ctx, sqlc.InsertBootstrapUserParams{ID: dtos.UUID(userID), Username: username, Email: email, PasswordHash: hash})
	if err != nil {
		return "", false, err
	}
	return value.String(), true, r.queries.BindBootstrapUser(ctx, value)
}

// Bootstrap runs under the same administration lock as account creation.
func (r *bootstrapRepository) requireBootstrapSlot(ctx context.Context) error {
	count, err := r.queries.CountSuperusers(ctx)
	if err != nil {
		return err
	}
	if count >= 2 {
		return domain.ErrSuperadminLimit
	}
	return nil
}
