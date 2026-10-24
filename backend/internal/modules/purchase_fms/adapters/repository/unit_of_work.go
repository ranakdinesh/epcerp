package repository

import (
	"context"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
)

// unitOfWork binds each workflow's narrow ports to the same caller transaction.
// The platform manager alone owns commit/rollback, including ambiguous commits.
type unitOfWork[R, C any] struct {
	manager *database.Manager
	bind    func(database.Connection) (R, C)
}

func newUnitOfWork[R, C any](b database.Beginner, bind func(database.Connection) (R, C)) *unitOfWork[R, C] {
	return &unitOfWork[R, C]{manager: database.NewManager(b), bind: bind}
}
func (u *unitOfWork[R, C]) Within(ctx context.Context, work func(context.Context, R, C) error) error {
	return u.manager.Within(ctx, func(ctx context.Context, c database.Connection) error {
		repository, scope := u.bind(c)
		return work(ctx, repository, scope)
	})
}
