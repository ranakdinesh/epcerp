package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type clientClearanceProgressReader struct{ c database.Connection }

func NewClientClearanceProgressReader(c database.Connection) contracts.ClientClearanceProgressReader {
	return &clientClearanceProgressReader{c}
}
func (r *clientClearanceProgressReader) Progress(ctx context.Context, p, pkg string, expected contracts.ClientClearanceProgressSource, upstreamCurrent bool) (domain.ClientClearanceProgress, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return domain.ClientClearanceProgress{}, errors.New("Evidence progress requires the consumer transaction")
	}
	for _, id := range []string{p, pkg} {
		if e := domain.ClientClearanceID(id); e != nil {
			return domain.ClientClearanceProgress{}, e
		}
	}
	if expected.ProjectID != p || expected.PackageID != pkg {
		return domain.ClientClearanceProgress{}, domain.ClientClearanceInvalid("source", "Evidence reporting source must match project and package")
	}
	record, err := (&clientClearanceRepository{q: NewStore(r.c)}).Get(ctx, p, pkg)
	if err != nil {
		return domain.ClientClearanceProgress{}, err
	}
	return domain.ClientClearanceProgressFact(record, expected, upstreamCurrent), nil
}
