package contracts

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type QuotationWorkTask = domain.QuotationWorkTask
type QuotationWorkReader interface {
	Own(context.Context, domain.Principal) ([]QuotationWorkTask, error)
}
