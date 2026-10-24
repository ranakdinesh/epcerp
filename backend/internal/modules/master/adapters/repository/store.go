// Package repository implements Master persistence and its published readers.
package repository

import (
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"
)

// Store binds every Master query to the supplied pool connection or transaction.
// Transaction ownership remains with database.Manager.
type Store struct{ Queries *sqlc.Queries }

func NewStore(connection database.Connection) *Store { return &Store{Queries: sqlc.New(connection)} }
