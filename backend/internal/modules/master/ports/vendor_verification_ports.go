package ports

import (
	"context"
	sharedIdentity "github.com/ranakdinesh/epcERP/internal/identity"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type VendorVerificationRepository interface {
	Get(context.Context, string) (*masterDomain.VendorVerificationReview, error)
	Save(context.Context, masterDomain.VendorVerificationReview, string) (*masterDomain.VendorVerificationReview, error)
	Journal(context.Context, masterDomain.VendorVerificationReview, string) error
}
type VendorVerificationEvidence interface {
	Snapshot(context.Context, string) (*masterDomain.VendorVerificationSnapshot, error)
}
type VendorVerificationUsers interface {
	Get(context.Context, string) (*sharedIdentity.Principal, error)
}
type VendorVerificationUnitOfWork interface {
	Within(context.Context, func(context.Context, VendorVerificationRepository, VendorVerificationEvidence, VendorVerificationUsers) error) error
}

type VendorVerificationValues interface {
	ID() string
	Fingerprint(masterDomain.VendorVerificationSnapshot) string
}
