package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type QuotationsProgress = domain.QuotationsProgress

// Internal saved-source projection. Consumer checks current document scope and
// project authorization on its shared transaction. No prices/budget values returned.
type QuotationsProgressReader interface {
	Progress(context.Context, string, string) (QuotationsProgress, error)
}
