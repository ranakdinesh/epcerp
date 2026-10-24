package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type SiteReceiptsProgress = domain.SiteReceiptsProgress
type SiteReceiptsProgressSource = domain.SiteReceiptsSourceFacts

// Consumer owns project authorization, locking and current upstream proof in its
// shared transaction. Reporting completion cannot authorize subsequent actions.
type SiteReceiptsProgressReader interface {
	Progress(context.Context, string, string, SiteReceiptsProgressSource, bool) (SiteReceiptsProgress, error)
}
