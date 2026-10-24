package ports

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type DispatchRepository interface {
	Get(context.Context, string, string) (*domain.DispatchRecord, error)
	Save(context.Context, domain.DispatchRecord) (domain.DispatchRecord, error)
	Event(context.Context, domain.DispatchEvent) error
	History(context.Context, string) ([]domain.DispatchEvent, error)
}
type DispatchContext interface {
	Access(context.Context, domain.Principal, string, string) (domain.DispatchAccess, error)
	Source(context.Context, string, string) (domain.DispatchSource, error)
}
type DispatchUnitOfWork interface {
	Within(context.Context, func(context.Context, DispatchRepository, DispatchContext) error) error
}
type DispatchAuthorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}
type DispatchValues interface {
	ID() string
	Now() domain.DispatchTimestamp
}
