package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type ClientClearanceProgress = domain.ClientClearanceProgress
type ClientClearanceProgressSource = domain.ClientClearanceSourceFacts

// Consumer owns project authorization, locking and current upstream proof in its
// shared transaction. Reporting completion cannot authorize subsequent actions.
type ClientClearanceProgressReader interface {
	Progress(context.Context, string, string, ClientClearanceProgressSource, bool) (ClientClearanceProgress, error)
}
