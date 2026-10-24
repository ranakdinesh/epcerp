package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	application "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type quotationsApprovalReader struct {
	c database.Connection
	p QuotationsProviders
	g contracts.IndentGroupsReader
	v ports.QuotationsValues
}

func NewQuotationsApprovalReader(c database.Connection, p QuotationsProviders, g contracts.IndentGroupsReader, v ports.QuotationsValues) contracts.QuotationsApprovalReader {
	return &quotationsApprovalReader{c, p, g, v}
}

func (r *quotationsApprovalReader) Source(ctx context.Context, project, bo string) (domain.QuotationsApprovalSource, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return domain.QuotationsApprovalSource{}, errors.New("Quotation approval source requires the consuming transaction")
	}
	if _, e := domain.QuotationsID(project); e != nil {
		return domain.QuotationsApprovalSource{}, e
	}
	if _, e := domain.QuotationsID(bo); e != nil {
		return domain.QuotationsApprovalSource{}, e
	}
	// All purchasing source writes serialize on this project. Keep the lock until
	// the consumer's decision, journal and Director notice have committed.
	p, e := r.p.Projects.Get(ctx, project, true)
	if e != nil {
		return domain.QuotationsApprovalSource{}, e
	}
	if p == nil {
		return domain.QuotationsApprovalSource{}, domain.QuotationsMissing
	}
	return application.QuotationsApprovalSource(ctx, &quotationsRepository{q: NewStore(r.c)}, &quotationsQuoteContext{r.p}, r, r.v.Now(), project, bo)
}

func (r *quotationsApprovalReader) Current(ctx context.Context, project, bo string) ([]domain.QuotationsApprovalGroup, error) {
	rows, e := r.g.Get(ctx, project, bo)
	if e != nil {
		return nil, e
	}
	out := []domain.QuotationsApprovalGroup{}
	if rows == nil || !rows.Active {
		return out, nil
	}
	for _, row := range rows.Groups {
		out = append(out, domain.QuotationsApprovalGroup{ID: row.ID, Name: row.Name, ScopeRevision: row.ScopeRevision})
	}
	return out, nil
}
