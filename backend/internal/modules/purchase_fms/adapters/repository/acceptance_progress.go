package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type acceptanceProgressReader struct {
	c      database.Connection
	source ports.AcceptanceContext
}

func NewAcceptanceProgressReader(c database.Connection, source ports.AcceptanceContext) contracts.AcceptanceProgressReader {
	return &acceptanceProgressReader{c, source}
}
func (r *acceptanceProgressReader) Progress(ctx context.Context, p, pkg string) (domain.AcceptanceProgress, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return domain.AcceptanceProgress{}, errors.New("Acceptance progress requires the consumer transaction")
	}
	for _, id := range []string{p, pkg} {
		if e := domain.AcceptanceID(id); e != nil {
			return domain.AcceptanceProgress{}, e
		}
	}
	repo := &acceptanceRepository{q: NewStore(r.c)}
	record, e := repo.Get(ctx, p, pkg)
	if e != nil {
		return domain.AcceptanceProgress{}, e
	}
	if record == nil || !record.Active {
		return domain.AcceptanceProgressFact(nil, domain.AcceptanceSource{}), nil
	}
	if r.source == nil {
		return domain.AcceptanceProgress{}, errors.New("Acceptance progress source is not configured")
	}
	current, e := r.source.Source(ctx, p, pkg)
	if e != nil {
		return domain.AcceptanceProgress{}, e
	}
	// Re-read after the source context's project lock serializes owner writes.
	record, e = repo.Get(ctx, p, pkg)
	if e != nil {
		return domain.AcceptanceProgress{}, e
	}
	return domain.AcceptanceProgressFact(record, current), nil
}
