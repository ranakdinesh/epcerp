package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type WorkflowPackage = domain.WorkflowPackage
type WorkflowSource = domain.WorkflowSource
type WorkflowActor = domain.Principal
type WorkflowError = domain.WorkflowError

// Internal transaction-bound source; the consumer authorizes its own capability.
type WorkflowSourceReader interface {
	Source(context.Context, string, string) (WorkflowPackage, WorkflowSource, error)
	// Inspect preserves current blockers for authorized historical reads. It grants no mutation authority.
	Inspect(context.Context, string, string) (WorkflowPackage, WorkflowSource, error)
	CanReadTask(context.Context, WorkflowActor, string, string, int) (bool, error)
	CanCancel(context.Context, WorkflowActor, string, string, int) (bool, error)
	CanWork(context.Context, WorkflowActor, string, string, int) (bool, error)
	CanConfigure(context.Context, WorkflowActor, string) (bool, error)
}
