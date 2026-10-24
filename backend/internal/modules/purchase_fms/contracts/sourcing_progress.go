package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type SourcingProgress = domain.SourcingProgress
type SourcingProgressSource = domain.SourcingSource

// Internal report consumer establishes live demand identity, project locking and
// authorization on the same transaction. Historical release is not AuthorityReader.
type SourcingProgressReader interface {
	Progress(context.Context, string, string, SourcingProgressSource, bool) (SourcingProgress, error)
}
