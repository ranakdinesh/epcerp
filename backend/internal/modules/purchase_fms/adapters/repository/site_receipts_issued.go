package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type siteReceiptsDeliveredReader struct {
	c database.Connection
	p SiteReceiptsProviders
}

func NewSiteReceiptsDeliveredReader(c database.Connection, p SiteReceiptsProviders) contracts.SiteReceiptsDeliveredReader {
	return &siteReceiptsDeliveredReader{c, p}
}
func (r *siteReceiptsDeliveredReader) Delivered(ctx context.Context, project, pkg string) (*contracts.SiteReceiptsDelivered, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, errors.New("Site material receipt requires the consuming transaction")
	}
	for _, v := range []string{project, pkg} {
		if e := domain.SiteReceiptsID(v); e != nil {
			return nil, e
		}
	}
	record, e := (&siteReceiptsRepository{q: NewStore(r.c)}).Get(ctx, project, pkg)
	if e != nil {
		return nil, e
	}
	source, e := NewSiteReceiptsContext(r.p).Source(ctx, project, pkg)
	if e != nil {
		return nil, e
	}
	return domain.SiteReceiptsDeliveredFact(record, source)
}
