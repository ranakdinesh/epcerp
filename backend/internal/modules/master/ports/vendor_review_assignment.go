package ports

import (
	"context"
	identity "github.com/ranakdinesh/epcERP/internal/identity"
	domain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type VendorReviewAssignmentRepository interface {
	LockSupplier(context.Context, string) (bool, error)
	Get(context.Context, string) (*domain.VendorReviewAssignment, error)
	Save(context.Context, domain.VendorReviewAssignment, string, string) (*domain.VendorReviewAssignment, error)
}
type VendorReviewAssignmentUsers interface {
	Get(context.Context, string) (*identity.Principal, error)
	Candidates(context.Context) ([]domain.VendorReviewer, error)
}
type VendorReviewAssignmentUnitOfWork interface {
	Within(context.Context, func(context.Context, VendorReviewAssignmentRepository, VendorReviewAssignmentUsers) error) error
}
