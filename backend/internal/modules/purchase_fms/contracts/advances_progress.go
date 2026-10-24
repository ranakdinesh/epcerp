package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type AdvancesProgress = domain.AdvancesProgress
type AdvancesProgressSource = domain.AdvancesSourceFacts

// Internal consumer establishes current upstream facts and project authorization in
// the same transaction. This projection grants no Finance or disbursement authority.
type AdvancesProgressReader interface {
	Progress(context.Context, string, string, AdvancesProgressSource, bool) (AdvancesProgress, error)
}
