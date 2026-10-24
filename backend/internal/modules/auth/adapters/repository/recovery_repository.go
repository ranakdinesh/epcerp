package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/dtos"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/sqlc"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/ports"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
)

type recoveryRepository struct{ *accountRepository }

func NewRecoveryUnitOfWork(b database.Beginner, e func(database.Connection) events.Writer) ports.UnitOfWork[ports.RecoveryRepository] {
	return &unitOfWork[ports.RecoveryRepository]{manager: database.NewManager(b), events: e, bind: func(s *Store) ports.RecoveryRepository { return &recoveryRepository{&accountRepository{s}} }}
}
func (r *recoveryRepository) FindEmail(ctx context.Context, email string) (*domain.User, error) {
	rows, e := r.queries.RecoveryUserByEmail(ctx, email)
	if e != nil {
		return nil, e
	}
	if len(rows) != 1 {
		return nil, nil
	}
	return r.user(ctx, rows[0])
}
func (r *recoveryRepository) RecoveryUserID(ctx context.Context, hash string) (string, error) {
	id, e := r.queries.RecoveryUserID(ctx, hash)
	if errors.Is(e, pgx.ErrNoRows) {
		return "", nil
	}
	return id.String(), e
}
func (r *recoveryRepository) ValidRecovery(ctx context.Context, hash string) (bool, error) {
	_, e := r.queries.RecoveryTokenValid(ctx, hash)
	if errors.Is(e, pgx.ErrNoRows) {
		return false, nil
	}
	return e == nil, e
}
func (r *recoveryRepository) RecentlyRequested(ctx context.Context, id string) (bool, error) {
	return r.queries.RecoveryRecentlyRequested(ctx, dtos.UUID(id))
}
func (r *recoveryRepository) CreateRecovery(ctx context.Context, hash, id string, expiry int64) error {
	return r.queries.InsertRecoveryToken(ctx, sqlc.InsertRecoveryTokenParams{TokenHash: hash, UserID: dtos.UUID(id), ExpiresAt: dtos.Stamp(domain.At(expiry))})
}
func (r *recoveryRepository) ConsumeRecoveries(ctx context.Context, id string) error {
	return r.queries.ConsumeRecoveryTokens(ctx, dtos.UUID(id))
}
