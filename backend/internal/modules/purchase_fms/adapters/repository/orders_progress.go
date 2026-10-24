package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type ordersProgressReader struct{ c database.Connection }

func NewOrdersProgressReader(c database.Connection) contracts.OrdersProgressReader {
	return &ordersProgressReader{c}
}
func (r *ordersProgressReader) Progress(ctx context.Context, p, pkg string) (domain.OrdersProgress, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return domain.OrdersProgress{}, errors.New("Order progress requires the consumer transaction")
	}
	for _, id := range []string{p, pkg} {
		if e := domain.OrdersID(id); e != nil {
			return domain.OrdersProgress{}, e
		}
	}
	record, e := (&ordersRepository{q: NewStore(r.c)}).Get(ctx, p, pkg)
	if e != nil {
		return domain.OrdersProgress{}, e
	}
	return domain.OrdersProgressFact(record, p, pkg), nil
}
