package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type TechnicalClearance = domain.TechnicalClearance
type TechnicalError = domain.TechnicalError

// Caller must authorize its own capability; implementation is bound to the same
// shared transaction and checks exact project/package, source revision and files.
type TechnicalClearanceReader interface {
	Clearance(context.Context, string, string) (*TechnicalClearance, error)
}
