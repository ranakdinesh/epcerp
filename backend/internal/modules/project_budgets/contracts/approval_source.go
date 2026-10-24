package contracts

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/domain"
)

type ApprovedSource = domain.ApprovedSource

// ApprovedReader is internal, transaction-bound evidence for authorized consumers.
// It grants no permission. Callers must enforce their own project-scoped use case.
// A nil result means there is no effective approved budget, not an empty approved budget.
type ApprovedReader interface {
	Approved(context.Context, string) (*ApprovedSource, error)
}

type Error = domain.Error
