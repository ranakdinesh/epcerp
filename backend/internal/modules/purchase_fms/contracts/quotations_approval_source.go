package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type QuotationsApprovalSource = domain.QuotationsApprovalSource

// ApprovalReader is internal and bound to the consuming transaction. The
// consumer must authorize its own project-scoped use case before calling it.
// This projection grants no authority and contains no budget or document bytes.
type QuotationsApprovalReader interface {
	Source(context.Context, string, string) (QuotationsApprovalSource, error)
}

type QuotationsError = domain.QuotationsError
