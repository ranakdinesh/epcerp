package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/sqlc"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

type AssignmentReader struct{ q *sqlc.Queries }

func NewAssignmentReader(c database.Connection) *AssignmentReader {
	return &AssignmentReader{NewStore(c, nil).queries}
}
func (r *AssignmentReader) Assignment(ctx context.Context, key string) (*domain.AssignmentCandidate, error) {
	v, e := r.q.AssignmentCandidate(ctx, dtos.UUID(key))
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	c, e := dtos.Assignment(v.ID.String(), v.Username, v.IsActive, v.IsSuperuser, v.PermissionCodes)
	c.Ready = v.Ready
	return &c, e
}
func (r *AssignmentReader) Assignments(ctx context.Context) ([]domain.AssignmentCandidate, error) {
	rows, e := r.q.AssignmentCandidates(ctx)
	if e != nil {
		return nil, e
	}
	result := []domain.AssignmentCandidate{}
	for _, v := range rows {
		c, e := dtos.Assignment(v.ID.String(), v.Username, v.IsActive, v.IsSuperuser, v.PermissionCodes)
		if e != nil {
			return nil, e
		}
		c.Ready = v.Ready
		result = append(result, c)
	}
	return result, nil
}
