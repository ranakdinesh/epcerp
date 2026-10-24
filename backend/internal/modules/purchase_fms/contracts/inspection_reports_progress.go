package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type InspectionReportsProgress = domain.InspectionReportsProgress
type InspectionReportsProgressSource = domain.InspectionReportsSourceFacts

// Consumer owns project authorization, locking and current upstream proof in its
// shared transaction. Reporting completion cannot authorize subsequent actions.
type InspectionReportsProgressReader interface {
	Progress(context.Context, string, string, InspectionReportsProgressSource, bool) (InspectionReportsProgress, error)
}
