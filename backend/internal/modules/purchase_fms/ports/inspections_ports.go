package ports

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type InspectionsRepository interface {
	Get(context.Context, string, string) (*domain.InspectionsRecord, error)
	Save(context.Context, domain.InspectionsRecord) (domain.InspectionsRecord, error)
	Event(context.Context, domain.InspectionsEvent) error
	History(context.Context, string) ([]domain.InspectionsEvent, error)
}
type InspectionsContext interface {
	Access(context.Context, domain.Principal, string, string) (domain.InspectionsAccess, error)
	Source(context.Context, string, string) (domain.InspectionsSource, error)
}
type InspectionsUnitOfWork interface {
	Within(context.Context, func(context.Context, InspectionsRepository, InspectionsContext) error) error
}
type InspectionsAuthorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}
type InspectionsValues interface {
	ID() string
	Now() domain.InspectionsTimestamp
}
