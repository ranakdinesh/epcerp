package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type siteReceiptsProgressReader struct{ c database.Connection }

func NewSiteReceiptsProgressReader(c database.Connection) contracts.SiteReceiptsProgressReader {
	return &siteReceiptsProgressReader{c}
}
func (r *siteReceiptsProgressReader) Progress(ctx context.Context, p, pkg string, expected contracts.SiteReceiptsProgressSource, upstreamCurrent bool) (domain.SiteReceiptsProgress, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return domain.SiteReceiptsProgress{}, errors.New("Evidence progress requires the consumer transaction")
	}
	for _, id := range []string{p, pkg} {
		if e := domain.SiteReceiptsID(id); e != nil {
			return domain.SiteReceiptsProgress{}, e
		}
	}
	if expected.ProjectID != p || expected.PackageID != pkg {
		return domain.SiteReceiptsProgress{}, domain.SiteReceiptsInvalid("source", "Evidence reporting source must match project and package")
	}
	record, err := (&siteReceiptsRepository{q: NewStore(r.c)}).Get(ctx, p, pkg)
	if err != nil {
		return domain.SiteReceiptsProgress{}, err
	}
	return domain.SiteReceiptsProgressFact(record, expected, upstreamCurrent), nil
}
