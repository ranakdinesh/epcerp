package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type EngineerConfirmationsProgress = domain.EngineerConfirmationsProgress
type EngineerConfirmationsProgressSource = domain.EngineerConfirmationsSourceFacts

// Consumer owns project authorization, locking and current upstream proof in its
// shared transaction. Reporting completion cannot authorize subsequent actions.
type EngineerConfirmationsProgressReader interface {
	Progress(context.Context, string, string, EngineerConfirmationsProgressSource, bool) (EngineerConfirmationsProgress, error)
}
