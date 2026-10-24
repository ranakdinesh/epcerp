package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type gRNsIssuedReader struct {
	c database.Connection
	p GRNsProviders
}

func NewGRNsIssuedReader(c database.Connection, p GRNsProviders) contracts.GRNsIssuedReader {
	return &gRNsIssuedReader{c, p}
}
func (r *gRNsIssuedReader) Issued(ctx context.Context, project, pkg string) (*contracts.GRNsIssued, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, errors.New("GRN inwarding requires the consuming transaction")
	}
	for _, v := range []string{project, pkg} {
		if e := domain.GRNsID(v); e != nil {
			return nil, e
		}
	}
	record, e := (&gRNsRepository{q: NewStore(r.c)}).Get(ctx, project, pkg)
	if e != nil {
		return nil, e
	}
	source, e := NewGRNsContext(r.p).Source(ctx, project, pkg)
	if e != nil {
		return nil, e
	}
	return domain.GRNsIssuedFact(record, source)
}
