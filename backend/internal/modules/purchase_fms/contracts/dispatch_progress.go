package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type DispatchProgress = domain.DispatchProgress
type DispatchProgressSource = domain.DispatchSourceFacts

// Consumer owns project authorization, locking and current upstream proof in its
// shared transaction. Reporting completion cannot authorize subsequent actions.
type DispatchProgressReader interface {
	Progress(context.Context, string, string, DispatchProgressSource, bool) (DispatchProgress, error)
}
