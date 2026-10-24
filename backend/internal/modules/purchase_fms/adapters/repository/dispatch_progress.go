package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type dispatchProgressReader struct{ c database.Connection }

func NewDispatchProgressReader(c database.Connection) contracts.DispatchProgressReader {
	return &dispatchProgressReader{c}
}
func (r *dispatchProgressReader) Progress(ctx context.Context, p, pkg string, expected contracts.DispatchProgressSource, upstreamCurrent bool) (domain.DispatchProgress, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return domain.DispatchProgress{}, errors.New("Evidence progress requires the consumer transaction")
	}
	for _, id := range []string{p, pkg} {
		if e := domain.DispatchID(id); e != nil {
			return domain.DispatchProgress{}, e
		}
	}
	if expected.ProjectID != p || expected.PackageID != pkg {
		return domain.DispatchProgress{}, domain.DispatchInvalid("source", "Evidence reporting source must match project and package")
	}
	record, err := (&dispatchRepository{q: NewStore(r.c)}).Get(ctx, p, pkg)
	if err != nil {
		return domain.DispatchProgress{}, err
	}
	return domain.DispatchProgressFact(record, expected, upstreamCurrent), nil
}
