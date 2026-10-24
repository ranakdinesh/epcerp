package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type sourcingProgressReader struct{ c database.Connection }

func NewSourcingProgressReader(c database.Connection) contracts.SourcingProgressReader {
	return &sourcingProgressReader{c}
}
func (r *sourcingProgressReader) Progress(ctx context.Context, p, pkg string, expected contracts.SourcingProgressSource, upstreamCurrent bool) (domain.SourcingProgress, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return domain.SourcingProgress{}, errors.New("Evidence progress requires the consumer transaction")
	}
	for _, id := range []string{p, pkg} {
		if e := domain.SourcingID(id); e != nil {
			return domain.SourcingProgress{}, e
		}
	}
	if expected.PackageID != pkg {
		return domain.SourcingProgress{}, domain.SourcingInvalid("source", "Evidence reporting source must match project and package")
	}
	record, err := (&sourcingRepository{q: NewStore(r.c)}).Get(ctx, p, pkg)
	if err != nil {
		return domain.SourcingProgress{}, err
	}
	return domain.SourcingProgressFacts(record, p, pkg, expected, upstreamCurrent), nil
}
