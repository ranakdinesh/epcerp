package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/sqlc"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

func (r *Store) Lock(ctx context.Context, key string) error {
	return r.queries.AdvisoryLock(ctx, key)
}

func (r *Store) user(ctx context.Context, row sqlc.AuthUser) (*domain.User, error) {
	user := dtos.User(row)
	if state, e := r.queries.CredentialEmailStatus(ctx, row.ID); e == nil {
		user.EmailStatus = state
	} else if !errors.Is(e, pgx.ErrNoRows) {
		return nil, e
	}
	rows, err := r.queries.ListUserRoles(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	for _, role := range rows {
		mapped, err := dtos.UserRole(role)
		if err != nil {
			return nil, err
		}
		user.Roles = append(user.Roles, mapped)
	}
	return user, nil
}

func (r *Store) User(ctx context.Context, userID string, lock bool) (*domain.User, error) {
	var row sqlc.AuthUser
	var err error
	if lock {
		row, err = r.queries.LockUser(ctx, dtos.UUID(userID))
	} else {
		row, err = r.queries.GetUser(ctx, dtos.UUID(userID))
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.user(ctx, row)
}

func (r *Store) role(ctx context.Context, row sqlc.AuthRole) (*domain.Role, error) {
	codes, err := r.queries.RolePermissions(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	return dtos.Role(row, codes), nil
}

func (r *Store) SetPassword(ctx context.Context, userID, hash, actor string) error {
	return r.queries.SetPassword(ctx, sqlc.SetPasswordParams{ID: dtos.UUID(userID), PasswordHash: hash, UpdatedByID: dtos.UUID(actor)})
}

func (r *Store) RevokeUserSessions(ctx context.Context, userID string) error {
	return r.queries.RevokeUserSessions(ctx, dtos.UUID(userID))
}

func (r *Store) GrantRolePermission(ctx context.Context, roleID, code string) error {
	return r.queries.GrantPermission(ctx, sqlc.GrantPermissionParams{RoleID: dtos.UUID(roleID), Code: code})
}

func (r *Store) AssignRole(ctx context.Context, userID, roleID string) error {
	return r.queries.AssignRole(ctx, sqlc.AssignRoleParams{UserID: dtos.UUID(userID), RoleID: dtos.UUID(roleID)})
}
