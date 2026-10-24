package ports

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
)

type OwnershipRepository interface {
	Get(context.Context, string) (domain.Intake, error)
	SaveOwner(context.Context, string, domain.OwnershipInput, string, string) (domain.Intake, error)
}
type OwnershipUsers interface {
	Get(context.Context, string) (*domain.Principal, error)
	List(context.Context) ([]domain.OwnershipCandidate, error)
}
type OwnershipUnit interface {
	Within(context.Context, func(context.Context, OwnershipRepository, OwnershipUsers) error) error
}
