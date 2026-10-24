package ports

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type AcceptanceRepository interface {
	Get(context.Context, string, string) (*domain.AcceptanceRecord, error)
	Save(context.Context, domain.AcceptanceRecord) (domain.AcceptanceRecord, error)
	Event(context.Context, domain.AcceptanceEvent) error
	History(context.Context, string) ([]domain.AcceptanceEvent, error)
}
type AcceptanceContext interface {
	Access(context.Context, domain.Principal, string, string) (domain.AcceptanceAccess, error)
	Source(context.Context, string, string) (domain.AcceptanceSource, error)
}
type AcceptanceUnitOfWork interface {
	Within(context.Context, func(context.Context, AcceptanceRepository, AcceptanceContext) error) error
}
type AcceptanceAuthorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}
type AcceptanceValues interface {
	ID() string
	Now() domain.AcceptanceTimestamp
}
