package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type clientClearanceClearedReader struct {
	c database.Connection
	p ClientClearanceProviders
}

func NewClientClearanceClearedReader(c database.Connection, p ClientClearanceProviders) contracts.ClientClearanceClearedReader {
	return &clientClearanceClearedReader{c, p}
}
func (r *clientClearanceClearedReader) Cleared(ctx context.Context, project, pkg string) (*contracts.ClientClearanceCleared, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, errors.New("Material clearance clearance requires the consuming transaction")
	}
	for _, v := range []string{project, pkg} {
		if e := domain.ClientClearanceID(v); e != nil {
			return nil, e
		}
	}
	record, e := (&clientClearanceRepository{q: NewStore(r.c)}).Get(ctx, project, pkg)
	if e != nil {
		return nil, e
	}
	source, e := NewClientClearanceContext(r.p).Source(ctx, project, pkg)
	if e != nil {
		return nil, e
	}
	return domain.ClientClearanceClearedFact(record, source)
}
