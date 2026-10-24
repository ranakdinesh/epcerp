package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type TechnicalProgress = domain.TechnicalProgress

// Internal reporting projection on the consumer transaction. The caller must
// authorize project reporting and establish parent-current from current LOI
// reporting evidence. This projection is never new commitment authority.
type TechnicalProgressReader interface {
	Progress(context.Context, string, string, bool) (TechnicalProgress, error)
}
