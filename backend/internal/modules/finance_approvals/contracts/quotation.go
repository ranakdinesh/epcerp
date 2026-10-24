package contracts

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/domain"
)

type QuotationRequest = domain.QuotationRequest
type QuotationReviewer interface {
	ReviewQuotation(context.Context, Actor, QuotationRequest) (string, error)
}
