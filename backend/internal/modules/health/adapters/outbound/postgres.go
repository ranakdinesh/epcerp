package outbound

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/health/adapters/outbound/internal/db"
)

// PostgresProbe keeps generated persistence types within the outbound adapter.
type PostgresProbe struct{ queries *db.Queries }

// DBTX accepts the shared pgx pool or a use-case-owned pgx transaction.
func NewPostgresProbe(connection db.DBTX) *PostgresProbe {
	return &PostgresProbe{queries: db.New(connection)}
}

func (p *PostgresProbe) Check(ctx context.Context) error {
	_, err := p.queries.CheckDatabase(ctx)
	return err
}
