package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type WorkflowAssignedTask = domain.WorkflowAssignedTask
type WorkflowTaskQuery = domain.WorkflowTaskQuery
type WorkflowTaskPage = domain.WorkflowTaskPage

// Personal task evidence only, current actor identity/live grants and shared transaction required.
// No portfolio user selector, mutation or new project access is granted.
type WorkflowTaskReader interface {
	Assigned(context.Context, WorkflowActor, WorkflowTaskQuery) (WorkflowTaskPage, error)
}
