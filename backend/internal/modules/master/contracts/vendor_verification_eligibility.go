package contracts

import (
	"context"
)

type VendorVerificationEligibility struct {
	CanInviteQuotation, CanGeneratePurchaseOrder bool
	Message                                      string
}

// Purchase FMS must call this port inside its own order transaction, not rely on browser state.
type VendorVerificationGate interface {
	Check(context.Context, string) (VendorVerificationEligibility, error)
	RequirePurchaseOrder(context.Context, string) error
}
