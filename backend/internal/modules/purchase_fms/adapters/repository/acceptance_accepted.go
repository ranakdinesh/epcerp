package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type acceptanceAcceptedReader struct {
	c database.Connection
	p AcceptanceProviders
}

func NewAcceptanceAcceptedReader(c database.Connection, p AcceptanceProviders) contracts.AcceptanceAcceptedReader {
	return &acceptanceAcceptedReader{c, p}
}
func (r *acceptanceAcceptedReader) Accepted(ctx context.Context, project, pkg string) (*contracts.AcceptanceAccepted, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, errors.New("Vendor acceptance evidence requires the consuming transaction")
	}
	for _, v := range []string{project, pkg} {
		if e := domain.AcceptanceID(v); e != nil {
			return nil, e
		}
	}
	record, e := (&acceptanceRepository{q: NewStore(r.c)}).Get(ctx, project, pkg)
	if e != nil {
		return nil, e
	}
	source, e := NewAcceptanceContext(r.p).Source(ctx, project, pkg)
	if e != nil {
		return nil, e
	}
	return domain.AcceptanceAcceptedFact(record, source)
}
