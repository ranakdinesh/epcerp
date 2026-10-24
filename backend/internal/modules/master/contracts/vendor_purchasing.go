package contracts

import (
	"context"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type VendorPurchaseCandidate = masterDomain.Vendor

// Internal procurement reader; caller owns project/resource authorization.
type VendorPurchaseDirectory interface {
	Candidates(context.Context) ([]VendorPurchaseCandidate, error)
}
