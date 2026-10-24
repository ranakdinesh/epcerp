package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type QualityProgress = domain.QualityProgress
type QualityProgressSource = domain.QualitySourceFacts

// Consumer owns project authorization, locking and current upstream proof in its
// shared transaction. Reporting completion cannot authorize subsequent actions.
type QualityProgressReader interface {
	Progress(context.Context, string, string, QualityProgressSource, bool) (QualityProgress, error)
}
