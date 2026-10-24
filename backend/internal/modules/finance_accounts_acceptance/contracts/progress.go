package contracts

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/domain"
)

type Progress = domain.Progress
type ProgressSource = domain.SourceFacts

// Consumer owns project authorization, locking and current upstream proof in its
// shared transaction. Reporting completion cannot authorize subsequent actions.
type ProgressReader interface {
	Progress(context.Context, string, string, ProgressSource, bool) (Progress, error)
}
