package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type GRNsProgress = domain.GRNsProgress
type GRNsProgressSource = domain.GRNsSourceFacts

// Consumer owns project authorization, locking and current upstream proof in its
// shared transaction. Reporting completion cannot authorize subsequent actions.
type GRNsProgressReader interface {
	Progress(context.Context, string, string, GRNsProgressSource, bool) (GRNsProgress, error)
}
