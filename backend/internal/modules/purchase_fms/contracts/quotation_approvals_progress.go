package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type QuotationApprovalsProgress = domain.QuotationApprovalsProgress

// Consumer authorizes whole-project reporting. No approval or commercial amounts
// are returned. Effective scope/budget/vendor proof is evaluated on its transaction.
type QuotationApprovalsProgressReader interface {
	Progress(context.Context, string, string) (QuotationApprovalsProgress, error)
}
