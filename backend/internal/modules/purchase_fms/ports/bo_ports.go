package ports

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type BORepository interface {
	List(context.Context, string) ([]domain.BORecord, error)
	Get(context.Context, string, string) (*domain.BORecord, error)
	Save(context.Context, domain.BORecord, string) (domain.BORecord, error)
	Event(context.Context, domain.BOEvent) error
	History(context.Context, string, string) ([]domain.BOEvent, error)
}
type BOContext interface {
	Project(context.Context, string) (domain.BOProject, error)
	Scope(context.Context, domain.Principal, string, string, bool) error
	Item(context.Context, string, string) (domain.BOSnapshot, error)
	DemandVersion(context.Context, string, domain.BOLine) (int32, error)
	Publish(context.Context, string, domain.BOLine, string, string) (string, error)
}
type BOUnitOfWork interface {
	Within(context.Context, func(context.Context, BORepository, BOContext) error) error
}
type BOAuthorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}
type BOValues interface {
	ID() string
	Now() domain.BOTimestamp
}

type BOPrinter interface {
	Render(context.Context, domain.BOPrint) ([]byte, error)
}
