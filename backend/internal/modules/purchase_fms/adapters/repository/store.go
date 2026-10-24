package repository

import (
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
)

// NewStore binds all purchase query sets to the supplied connection.
func NewStore(c database.Connection) *sqlc.Queries { return sqlc.New(c) }
