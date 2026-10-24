package ports

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type HoldsRepository interface {
	List(context.Context, string) ([]domain.HoldsRecord, error)
	Get(context.Context, string, string) (*domain.HoldsRecord, error)
	Save(context.Context, domain.HoldsRecord, string) (domain.HoldsRecord, error)
	Event(context.Context, domain.HoldsEvent) error
	History(context.Context, string, string) ([]domain.HoldsEvent, error)
}
type HoldsContext interface {
	Scope(context.Context, domain.Principal, string, string, bool) error
	Validate(context.Context, string, domain.HoldsCreate) error
}
type HoldsUnitOfWork interface {
	Within(context.Context, func(context.Context, HoldsRepository, HoldsContext) error) error
}
type HoldsAuthorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}
type HoldsValues interface {
	ID() string
	Now() domain.HoldsTimestamp
}
