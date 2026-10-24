package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type QuotationApprovalsAttempt = domain.QuotationApprovalsAttempt
type QuotationApprovalsCommercialLine = domain.QuotationApprovalsCommercialLine
type QuotationApprovalsError = domain.QuotationApprovalsError

// ApprovedReader is a transaction-bound downstream gate, not an authority grant.
// It rechecks current groups, quote eligibility, approved budget and supplier evidence.
type QuotationApprovalsApprovedReader interface {
	Commercial(context.Context, string, string) ([]QuotationApprovalsCommercialLine, error)
	Approved(context.Context, string, string) ([]QuotationApprovalsAttempt, error)
}

// GroupApprovedReader permits independently cleared groups to prepare sourcing;
// technical execution still requires ProjectClearance.
type QuotationGroupApprovedReader interface {
	ApprovedGroups(context.Context, string, string) ([]QuotationApprovalsAttempt, error)
	CommercialGroups(context.Context, string, string) ([]QuotationApprovalsCommercialLine, error)
}
type QuotationProjectClearance interface {
	Ready(context.Context, string) (bool, error)
}
