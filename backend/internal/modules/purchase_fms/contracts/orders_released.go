package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type OrdersReleasedOrder = domain.OrdersOrder
type OrdersError = domain.OrdersError

// ReleasedReader is an internal historical fact provider on the consumer transaction.
// A released order is immutable; this provider grants no user access or live gate.
type OrdersReleasedReader interface {
	Released(context.Context, string, string) (*OrdersReleasedOrder, error)
}
