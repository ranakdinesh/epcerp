package repository

import (
	"context"
	"errors"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/dtos"

	pgx "github.com/jackc/pgx/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type ClientIdentityReader struct{ q *db.Queries }

func NewClientIdentityReader(c database.Connection) *ClientIdentityReader {
	return &ClientIdentityReader{NewStore(c).Queries}
}
func (r *ClientIdentityReader) Identity(ctx context.Context, key string) (*masterDomain.ClientDirectoryEntry, error) {
	v, e := r.q.ClientsClientIdentity(ctx, masterDTO.UUID(key))
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	return &masterDomain.ClientDirectoryEntry{ID: v.ID.String(), Name: v.DisplayName, Code: v.ClientCode, Active: v.IsActive}, nil
}
func (r *ClientIdentityReader) Identities(ctx context.Context) ([]masterDomain.ClientDirectoryEntry, error) {
	rows, e := r.q.ClientsClientIdentities(ctx)
	if e != nil {
		return nil, e
	}
	result := []masterDomain.ClientDirectoryEntry{}
	for _, v := range rows {
		result = append(result, masterDomain.ClientDirectoryEntry{ID: v.ID.String(), Name: v.DisplayName, Code: v.ClientCode, Active: v.IsActive})
	}
	return result, nil
}
