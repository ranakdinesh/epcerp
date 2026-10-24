package contracts

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/domain"
)

type Actor = domain.Actor
type BudgetRequest = domain.BudgetRequest
type Error = domain.Error

// BudgetApprover owns Finance decisions. The caller owns validation of its current
// budget and supplies this provider on the same shared transaction/context.
type BudgetApprover interface {
	ApproveBudget(context.Context, Actor, BudgetRequest) (string, error)
}
