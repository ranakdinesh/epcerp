package ports

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type ClientClearanceRepository interface {
	Get(context.Context, string, string) (*domain.ClientClearanceRecord, error)
	Save(context.Context, domain.ClientClearanceRecord) (domain.ClientClearanceRecord, error)
	Event(context.Context, domain.ClientClearanceEvent) error
	History(context.Context, string) ([]domain.ClientClearanceEvent, error)
}
type ClientClearanceContext interface {
	Access(context.Context, domain.Principal, string, string) (domain.ClientClearanceAccess, error)
	Source(context.Context, string, string) (domain.ClientClearanceSource, error)
}
type ClientClearanceUnitOfWork interface {
	Within(context.Context, func(context.Context, ClientClearanceRepository, ClientClearanceContext) error) error
}
type ClientClearanceAuthorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}
type ClientClearanceValues interface {
	ID() string
	Now() domain.ClientClearanceTimestamp
}
