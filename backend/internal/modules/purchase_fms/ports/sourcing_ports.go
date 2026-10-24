package ports

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type SourcingRepository interface {
	Get(context.Context, string, string) (*domain.SourcingCase, error)
	LockAllocation(context.Context, string, string, string, string) (domain.SourcingBalances, error)
	StoreAllocation(context.Context, domain.SourcingCase, *domain.SourcingProof, domain.SourcingAllocation) error
	Save(context.Context, domain.SourcingCase, string) (domain.SourcingCase, error)
	History(context.Context, string, string) ([]domain.SourcingEvent, error)
	Event(context.Context, domain.SourcingEvent) error
}
type SourcingContext interface {
	Access(context.Context, domain.Principal, string, string) (domain.SourcingAccess, error)
	Source(context.Context, string, string) (domain.SourcingSource, error)
	Suppliers(context.Context, string, string) ([]domain.SourcingSupplier, error)
	Evidence(context.Context, string, string) ([]domain.SourcingApprovedEvidence, error)
	Approvers(context.Context, string) ([]domain.SourcingMember, error)
	// VerifySelection must prove current approved exact source, vendor, quantity and charges. It grants no actor authority.
	VerifySelection(context.Context, string, string, domain.SourcingOffer) (*domain.SourcingProof, error)
}
type SourcingUnitOfWork interface {
	Within(context.Context, func(context.Context, SourcingRepository, SourcingContext) error) error
}
type SourcingAuthorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}
type SourcingValues interface {
	ID() string
	Now() domain.SourcingTimestamp
}
