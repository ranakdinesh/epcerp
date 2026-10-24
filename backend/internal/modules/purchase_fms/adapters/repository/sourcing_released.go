package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type sourcingReleasedReader struct{ c database.Connection }

func NewSourcingReleasedReader(c database.Connection) contracts.SourcingReleasedReader {
	return &sourcingReleasedReader{c}
}
func (r *sourcingReleasedReader) Released(ctx context.Context, p, id string) (*contracts.SourcingReleased, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, errors.New("Released LOI evidence requires the consuming transaction")
	}
	for _, v := range []string{p, id} {
		if e := domain.SourcingID(v); e != nil {
			return nil, e
		}
	}
	record, e := (&sourcingRepository{q: NewStore(r.c)}).Get(ctx, p, id)
	if e != nil || record == nil {
		return nil, e
	}
	return domain.SourcingReleasedFact(*record)
}
