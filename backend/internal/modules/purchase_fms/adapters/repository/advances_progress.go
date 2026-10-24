package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type advancesProgressReader struct{ c database.Connection }

func NewAdvancesProgressReader(c database.Connection) contracts.AdvancesProgressReader {
	return &advancesProgressReader{c}
}
func (r *advancesProgressReader) Progress(ctx context.Context, p, pkg string, expected contracts.AdvancesProgressSource, upstreamCurrent bool) (domain.AdvancesProgress, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return domain.AdvancesProgress{}, errors.New("Advance progress requires the consumer transaction")
	}
	for _, id := range []string{p, pkg} {
		if e := domain.AdvancesID(id); e != nil {
			return domain.AdvancesProgress{}, e
		}
	}
	if expected.ProjectID != p || expected.PackageID != pkg {
		return domain.AdvancesProgress{}, domain.AdvancesInvalid("source", "Advance reporting source must match project and package")
	}
	record, err := (&advancesRepository{q: NewStore(r.c)}).Get(ctx, p, pkg)
	if err != nil {
		return domain.AdvancesProgress{}, err
	}
	return domain.AdvancesProgressFact(record, expected, upstreamCurrent, (Values{}).Now()), nil
}
