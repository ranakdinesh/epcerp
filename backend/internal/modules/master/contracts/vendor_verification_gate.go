package contracts

import (
	"context"
)

type VendorVerificationSourceStatus struct{ State, Fingerprint, Message string }

// SourceReader grants no authority; consumers enforce their own scoped use case.
type VendorVerificationSourceReader interface {
	Source(context.Context, string) (VendorVerificationSourceStatus, error)
}
