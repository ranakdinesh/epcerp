package ports

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type GRNsRepository interface {
	Get(context.Context, string, string) (*domain.GRNsRecord, error)
	Save(context.Context, domain.GRNsRecord) (domain.GRNsRecord, error)
	Event(context.Context, domain.GRNsEvent) error
	History(context.Context, string) ([]domain.GRNsEvent, error)
}
type GRNsContext interface {
	Access(context.Context, domain.Principal, string, string) (domain.GRNsAccess, error)
	Source(context.Context, string, string) (domain.GRNsSource, error)
}
type GRNsUnitOfWork interface {
	Within(context.Context, func(context.Context, GRNsRepository, GRNsContext) error) error
}
type GRNsAuthorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}
type GRNsValues interface {
	ID() string
	Now() domain.GRNsTimestamp
}
