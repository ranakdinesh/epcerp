package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type dispatchDispatchedReader struct {
	c database.Connection
	p DispatchProviders
}

func NewDispatchDispatchedReader(c database.Connection, p DispatchProviders) contracts.DispatchedReader {
	return &dispatchDispatchedReader{c, p}
}
func (r *dispatchDispatchedReader) Dispatched(ctx context.Context, project, pkg string) (*contracts.Dispatched, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, errors.New("Material dispatch dispatch requires the consuming transaction")
	}
	for _, v := range []string{project, pkg} {
		if e := domain.DispatchID(v); e != nil {
			return nil, e
		}
	}
	record, e := (&dispatchRepository{q: NewStore(r.c)}).Get(ctx, project, pkg)
	if e != nil {
		return nil, e
	}
	source, e := NewDispatchContext(r.p).Source(ctx, project, pkg)
	if e != nil {
		return nil, e
	}
	return domain.DispatchedFact(record, source)
}
