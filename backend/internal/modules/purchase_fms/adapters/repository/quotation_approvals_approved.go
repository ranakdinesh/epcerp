package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	application "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type quotationApprovalsApprovedReader struct {
	c database.Connection
	p QuotationApprovalsProviders
}

func NewQuotationApprovalsApprovedReader(c database.Connection, p QuotationApprovalsProviders) contracts.QuotationApprovalsApprovedReader {
	return &quotationApprovalsApprovedReader{c, p}
}
func (r *quotationApprovalsApprovedReader) Approved(ctx context.Context, p, b string) ([]domain.QuotationApprovalsAttempt, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, errors.New("Approved quotation gate requires the consuming transaction")
	}
	if _, e := domain.QuotationApprovalsID(p); e != nil {
		return nil, e
	}
	if _, e := domain.QuotationApprovalsID(b); e != nil {
		return nil, e
	}
	return application.QuotationApprovalsApprovedSelections(ctx, &quotationApprovalsRepository{q: NewStore(r.c)}, &quotationApprovalsSourceContext{r.p}, p, b)
}

func (r *quotationApprovalsApprovedReader) Commercial(ctx context.Context, p, b string) ([]domain.QuotationApprovalsCommercialLine, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, errors.New("Approved commercial gate requires the consuming transaction")
	}
	if _, e := domain.QuotationApprovalsID(p); e != nil {
		return nil, e
	}
	if _, e := domain.QuotationApprovalsID(b); e != nil {
		return nil, e
	}
	return application.QuotationApprovalsCurrentCommercialSelections(ctx, &quotationApprovalsRepository{q: NewStore(r.c)}, &quotationApprovalsSourceContext{r.p}, p, b)
}

func (r *quotationApprovalsApprovedReader) ApprovedGroups(ctx context.Context, p, b string) ([]domain.QuotationApprovalsAttempt, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, errors.New("Group approval requires consuming transaction")
	}
	return application.QuotationApprovalsApprovedGroups(ctx, &quotationApprovalsRepository{q: NewStore(r.c)}, &quotationApprovalsSourceContext{r.p}, p, b)
}
func (r *quotationApprovalsApprovedReader) CommercialGroups(ctx context.Context, p, b string) ([]domain.QuotationApprovalsCommercialLine, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, errors.New("Group commercial approval requires consuming transaction")
	}
	return application.QuotationApprovalsCommercialGroups(ctx, &quotationApprovalsRepository{q: NewStore(r.c)}, &quotationApprovalsSourceContext{r.p}, p, b)
}
func NewQuotationProjectClearance(c database.Connection, p QuotationApprovalsProviders) contracts.QuotationProjectClearance {
	return &quotationApprovalsApprovedReader{c, p}
}
func (r *quotationApprovalsApprovedReader) Ready(ctx context.Context, p string) (bool, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return false, errors.New("Project clearance requires consuming transaction")
	}
	bos, e := NewBOReader(r.c).List(ctx, p)
	if e != nil {
		return false, e
	}
	found := false
	for _, bo := range bos {
		if !bo.Active {
			continue
		}
		found = true
		source, e := (&quotationApprovalsSourceContext{r.p}).Source(ctx, p, bo.ID)
		if e != nil {
			return false, e
		}
		record, e := (&quotationApprovalsRepository{q: NewStore(r.c)}).Get(ctx, p, bo.ID)
		if e != nil {
			return false, e
		}
		if record == nil {
			return false, nil
		}
		_, _, ready := domain.QuotationApprovalsEvaluate(record.Attempts, source)
		if !ready {
			return false, nil
		}
	}
	return found, nil
}
