package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type gRNsProgressReader struct{ c database.Connection }

func NewGRNsProgressReader(c database.Connection) contracts.GRNsProgressReader {
	return &gRNsProgressReader{c}
}
func (r *gRNsProgressReader) Progress(ctx context.Context, p, pkg string, expected contracts.GRNsProgressSource, upstreamCurrent bool) (domain.GRNsProgress, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return domain.GRNsProgress{}, errors.New("Evidence progress requires the consumer transaction")
	}
	for _, id := range []string{p, pkg} {
		if e := domain.GRNsID(id); e != nil {
			return domain.GRNsProgress{}, e
		}
	}
	if expected.ProjectID != p || expected.PackageID != pkg {
		return domain.GRNsProgress{}, domain.GRNsInvalid("source", "Evidence reporting source must match project and package")
	}
	record, err := (&gRNsRepository{q: NewStore(r.c)}).Get(ctx, p, pkg)
	if err != nil {
		return domain.GRNsProgress{}, err
	}
	return domain.GRNsProgressFact(record, expected, upstreamCurrent), nil
}
