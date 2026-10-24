package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type qualityProgressReader struct{ c database.Connection }

func NewQualityProgressReader(c database.Connection) contracts.QualityProgressReader {
	return &qualityProgressReader{c}
}
func (r *qualityProgressReader) Progress(ctx context.Context, p, pkg string, expected contracts.QualityProgressSource, upstreamCurrent bool) (domain.QualityProgress, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return domain.QualityProgress{}, errors.New("Evidence progress requires the consumer transaction")
	}
	for _, id := range []string{p, pkg} {
		if e := domain.QualityID(id); e != nil {
			return domain.QualityProgress{}, e
		}
	}
	if expected.ProjectID != p || expected.PackageID != pkg {
		return domain.QualityProgress{}, domain.QualityInvalid("source", "Evidence reporting source must match project and package")
	}
	record, err := (&qualityRepository{q: NewStore(r.c)}).Get(ctx, p, pkg)
	if err != nil {
		return domain.QualityProgress{}, err
	}
	return domain.QualityProgressFact(record, expected, upstreamCurrent, domain.QualityToday((Values{}).Now())), nil
}
