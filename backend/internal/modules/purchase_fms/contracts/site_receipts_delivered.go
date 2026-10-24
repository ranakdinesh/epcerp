package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type SiteReceiptsDelivered = domain.SiteReceiptsRecord
type SiteReceiptsError = domain.SiteReceiptsError

// Current physical arrival evidence; condition may be damaged or quarantined.
// Consumer authorizes its own GRN workflow and determines stock acceptance separately.
type SiteReceiptsDeliveredReader interface {
	Delivered(context.Context, string, string) (*SiteReceiptsDelivered, error)
}
