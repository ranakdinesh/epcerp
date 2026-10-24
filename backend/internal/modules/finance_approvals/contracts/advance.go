package contracts

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/domain"
)

type AdvanceRequest = domain.AdvanceRequest

// AdvanceReviewer must be bound to the consuming advance transaction. Finance
// records its decision and commits a mandatory Director notice through its owner.
type AdvanceReviewer interface {
	ReviewAdvance(context.Context, Actor, AdvanceRequest) (string, error)
}
