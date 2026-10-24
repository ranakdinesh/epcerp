package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type WorkflowProgressPage = domain.WorkflowProgressPage

// Internal project/demand-bound, paged projection on the consuming transaction.
// The report must authorize the project first. No task/portfolio grant is implied.
type WorkflowProgressReader interface {
	Page(context.Context, string, []string, int32, int32) (WorkflowProgressPage, error)
}
