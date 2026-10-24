package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type Dispatched = domain.DispatchRecord
type DispatchError = domain.DispatchError

// Current dispatched evidence; consumer authorizes its own site/receipt workflow.
type DispatchedReader interface {
	Dispatched(context.Context, string, string) (*Dispatched, error)
}
