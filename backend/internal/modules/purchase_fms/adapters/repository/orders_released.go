package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type ordersReleasedReader struct{ c database.Connection }

func NewOrdersReleasedReader(c database.Connection) contracts.OrdersReleasedReader {
	return &ordersReleasedReader{c}
}
func (r *ordersReleasedReader) Released(ctx context.Context, project, pkg string) (*contracts.OrdersReleasedOrder, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, errors.New("Released PO evidence requires the consuming transaction")
	}
	for _, v := range []string{project, pkg} {
		if err := domain.OrdersID(v); err != nil {
			return nil, err
		}
	}
	row, err := (&ordersRepository{q: NewStore(r.c)}).Get(ctx, project, pkg)
	if err != nil {
		return nil, err
	}
	return domain.OrdersReleasedFact(row, project, pkg)
}
