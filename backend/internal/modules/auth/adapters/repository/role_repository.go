package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/sqlc"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

type roleRepository struct{ *Store }

func (r *roleRepository) Roles(ctx context.Context) ([]domain.Role, error) {
	rows, err := r.queries.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	result := []domain.Role{}
	for _, row := range rows {
		role, err := r.role(ctx, row)
		if err != nil {
			return nil, err
		}
		result = append(result, *role)
	}
	return result, nil
}

func (r *roleRepository) Role(ctx context.Context, roleID string) (*domain.Role, error) {
	row, err := r.queries.GetRole(ctx, dtos.UUID(roleID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.role(ctx, row)
}

func (r *roleRepository) PermissionCodes(ctx context.Context) ([]string, error) {
	return r.queries.RegisteredPermissions(ctx)
}

func (r *roleRepository) CreateRole(ctx context.Context, roleID string, request domain.RoleCreate, actor string) (*domain.Role, error) {
	row, err := r.queries.InsertRole(ctx, sqlc.InsertRoleParams{ID: dtos.UUID(roleID), Name: request.Name, Description: request.Description, CreatedByID: dtos.UUID(actor)})
	if unique(err) {
		return nil, domain.ErrRoleConflict
	}
	if err != nil {
		return nil, err
	}
	for _, code := range request.PermissionCodes {
		if err = r.GrantRolePermission(ctx, row.ID.String(), code); err != nil {
			return nil, err
		}
	}
	return r.role(ctx, row)
}

func (r *roleRepository) UpdateRole(ctx context.Context, roleID string, request domain.RoleUpdate, actor string) (*domain.Role, error) {
	err := r.queries.UpdateRole(ctx, sqlc.UpdateRoleParams{ID: dtos.UUID(roleID), Description: dtos.Nullable(request.Description), Active: dtos.Boolean(request.Active), ActorID: dtos.UUID(actor)})
	if err != nil {
		return nil, err
	}
	if request.PermissionCodes != nil {
		if err = r.queries.ClearRolePermissions(ctx, dtos.UUID(roleID)); err != nil {
			return nil, err
		}
		for _, code := range *request.PermissionCodes {
			if err = r.GrantRolePermission(ctx, roleID, code); err != nil {
				return nil, err
			}
		}
	}
	return r.Role(ctx, roleID)
}
