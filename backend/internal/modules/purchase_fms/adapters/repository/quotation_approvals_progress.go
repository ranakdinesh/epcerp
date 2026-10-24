package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type quotationApprovalsProgressReader struct {
	c database.Connection
	p QuotationApprovalsProviders
}

func NewQuotationApprovalsProgressReader(c database.Connection, p QuotationApprovalsProviders) contracts.QuotationApprovalsProgressReader {
	return &quotationApprovalsProgressReader{c, p}
}
func (r *quotationApprovalsProgressReader) Progress(ctx context.Context, p, b string) (domain.QuotationApprovalsProgress, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return domain.QuotationApprovalsProgress{}, errors.New("Approval progress requires the consumer transaction")
	}
	for _, id := range []string{p, b} {
		if _, e := domain.QuotationApprovalsID(id); e != nil {
			return domain.QuotationApprovalsProgress{}, e
		}
	}
	source, e := (&quotationApprovalsSourceContext{r.p}).Source(ctx, p, b)
	if e != nil {
		return domain.QuotationApprovalsProgress{}, e
	}
	if source.ProjectID != p || source.BOID != b {
		return domain.QuotationApprovalsProgress{}, domain.QuotationApprovalsInvalid("source", "Approval progress source scope changed")
	}
	record, e := (&quotationApprovalsRepository{q: NewStore(r.c)}).Get(ctx, p, b)
	if e != nil {
		return domain.QuotationApprovalsProgress{}, e
	}
	return domain.QuotationApprovalsProgressFacts(record, source)
}
