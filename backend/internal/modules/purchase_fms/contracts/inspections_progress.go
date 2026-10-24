package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type InspectionsProgress = domain.InspectionsProgress
type InspectionsProgressSource = domain.InspectionsSourceFacts

// Consumer owns project authorization, locking and current upstream proof in its
// shared transaction. Reporting completion cannot authorize subsequent actions.
type InspectionsProgressReader interface {
	Progress(context.Context, string, string, InspectionsProgressSource, bool) (InspectionsProgress, error)
}
