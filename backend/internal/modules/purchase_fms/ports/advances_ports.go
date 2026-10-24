package ports

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type AdvancesRepository interface {
	Get(context.Context, string, string) (*domain.AdvancesRecord, error)
	Save(context.Context, domain.AdvancesRecord) (domain.AdvancesRecord, error)
	Event(context.Context, domain.AdvancesEvent) error
	History(context.Context, string) ([]domain.AdvancesEvent, error)
}
type AdvancesContext interface {
	Access(context.Context, domain.Principal, string, string) (domain.AdvancesAccess, error)
	Source(context.Context, string, string) (domain.AdvancesSource, error)
}
type AdvancesFinance interface {
	Decide(context.Context, domain.Principal, domain.AdvancesRecord, string, string) (string, error)
}
type AdvancesUnitOfWork interface {
	Within(context.Context, func(context.Context, AdvancesRepository, AdvancesContext, AdvancesFinance) error) error
}
type AdvancesAuthorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}
type AdvancesValues interface {
	ID() string
	Now() domain.AdvancesTimestamp
}

type AdvancesQueueRepository interface {
	Queue(context.Context, int32) ([]domain.AdvancesRecord, error)
}
type AdvancesQueueContext interface {
	FinanceRead(context.Context, domain.Principal, string) (bool, error)
	ProjectLabel(context.Context, string) (string, string, error)
}
