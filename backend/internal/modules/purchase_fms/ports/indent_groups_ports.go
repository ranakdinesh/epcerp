package ports

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type IndentGroupsRepository interface {
	Get(context.Context, string, string) (*domain.IndentGroupsRecord, error)
	Save(context.Context, domain.IndentGroupsRecord, string) (domain.IndentGroupsRecord, error)
	Event(context.Context, domain.IndentGroupsEvent) error
	History(context.Context, string, string) ([]domain.IndentGroupsEvent, error)
}
type IndentGroupsContext interface {
	Scope(context.Context, domain.Principal, string, string, bool) (bool, error)
	Source(context.Context, string, string) (domain.IndentGroupsSource, error)
	Vendors(context.Context) ([]domain.IndentGroupsVendor, error)
}
type IndentGroupsUnitOfWork interface {
	Within(context.Context, func(context.Context, IndentGroupsRepository, IndentGroupsContext) error) error
}
type IndentGroupsAuthorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}
type IndentGroupsValues interface {
	ID() string
	Now() domain.IndentGroupsTimestamp
	NameKey(string) string
}

type IndentGroupsFreezeGuard interface {
	PendingGroups(context.Context, string, string) ([]string, error)
}
