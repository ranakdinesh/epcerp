package contracts

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/domain"
)

type Accepted = domain.Record
type Error = domain.Error

// Current final Accounts acceptance evidence; consumer authorizes its own workflow.
type AcceptedReader interface {
	Accepted(context.Context, string, string) (*Accepted, error)
}
