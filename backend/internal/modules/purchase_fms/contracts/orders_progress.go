package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type OrdersProgress = domain.OrdersProgress

// Reporting only; caller authorizes project scope on this transaction. Released
// historical evidence is distinct from current eligibility to release another PO.
type OrdersProgressReader interface {
	Progress(context.Context, string, string) (OrdersProgress, error)
}
