package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/sqlc"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
)

type BundleDirectory struct{ q *sqlc.Queries }

func NewBundleDirectory(c database.Connection) *BundleDirectory {
	return &BundleDirectory{NewStore(c, nil).queries}
}
func (d *BundleDirectory) Get(ctx context.Context, key string) (*contracts.Bundle, error) {
	r, e := d.q.LockAssignmentBundle(ctx, dtos.UUID(key))
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	p, e := d.q.RolePermissions(ctx, r.ID)
	bundle := dtos.Bundle(r.ID.String(), r.Name, r.IsActive, p)
	return &bundle, e
}
func (d *BundleDirectory) List(ctx context.Context) ([]contracts.Bundle, error) {
	rows, e := d.q.ListRoles(ctx)
	if e != nil {
		return nil, e
	}
	out := []contracts.Bundle{}
	for _, r := range rows {
		p, e := d.q.RolePermissions(ctx, r.ID)
		if e != nil {
			return nil, e
		}
		out = append(out, dtos.Bundle(r.ID.String(), r.Name, r.IsActive, p))
	}
	return out, nil
}
func (d *BundleDirectory) Member(ctx context.Context, user, bundle string) (bool, error) {
	rows, e := d.q.ListUserRoles(ctx, dtos.UUID(user))
	if e != nil {
		return false, e
	}
	for _, r := range rows {
		if r.ID.String() == bundle && r.IsActive {
			return true, nil
		}
	}
	return false, nil
}

func (d *BundleDirectory) Assigned(ctx context.Context, user string) ([]contracts.Bundle, error) {
	rows, e := d.q.ListUserRoles(ctx, dtos.UUID(user))
	if e != nil {
		return nil, e
	}
	out := []contracts.Bundle{}
	for _, r := range rows {
		if !r.IsActive {
			continue
		}
		p := []string{}
		if e = json.Unmarshal(r.PermissionCodes, &p); e != nil {
			return nil, e
		}
		out = append(out, dtos.Bundle(r.ID.String(), r.Name, r.IsActive, p))
	}
	return out, nil
}
