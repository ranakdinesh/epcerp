package repository

import (
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/sqlc"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
)

type Store struct {
	work    map[string]contracts.WorkSource
	queries *sqlc.Queries
	events  events.Writer
}

func NewStore(c database.Connection, events events.Writer) *Store {
	return &Store{queries: sqlc.New(c), events: events}
}
