package ports

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/domain"
)

type Repository interface {
	Get(context.Context, string) (*domain.Budget, error)
	Save(context.Context, domain.Budget, string) (domain.Budget, error)
	Event(context.Context, domain.Event) error
	History(context.Context, string) ([]domain.Event, error)
}
type Context interface {
	Scope(context.Context, domain.Actor, string, string, bool) (bool, error)
	Source(context.Context, string) (domain.Scope, error)
	Capabilities(context.Context, domain.Actor, string) (domain.Capabilities, error)
}

// Approvals is implemented by a transaction-bound Finance publication. Budget
// persistence must never manufacture a financial decision or Director notice.
type Approvals interface {
	DecideBudget(context.Context, domain.Actor, domain.Budget, string) (string, error)
}
type UnitOfWork interface {
	Within(context.Context, func(context.Context, Repository, Context, Approvals) error) error
}
type Authorizer interface {
	Resolve(context.Context, string) (domain.Actor, error)
}
type Values interface {
	ID() string
	Now() domain.Timestamp
}
