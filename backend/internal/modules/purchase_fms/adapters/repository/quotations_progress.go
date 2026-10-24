package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type quotationsProgressReader struct{ c database.Connection }

func NewQuotationsProgressReader(c database.Connection) contracts.QuotationsProgressReader {
	return &quotationsProgressReader{c}
}
func (r *quotationsProgressReader) Progress(ctx context.Context, p, b string) (domain.QuotationsProgress, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return domain.QuotationsProgress{}, errors.New("Quotation progress requires the consumer transaction")
	}
	for _, id := range []string{p, b} {
		if _, e := domain.QuotationsID(id); e != nil {
			return domain.QuotationsProgress{}, e
		}
	}
	record, e := (&quotationsRepository{q: NewStore(r.c)}).Get(ctx, p, b)
	if e != nil {
		return domain.QuotationsProgress{}, e
	}
	return domain.QuotationsProgressFacts(record, p, b)
}
