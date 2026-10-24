package ports

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/domain"
)

type Repository interface {
	Get(context.Context, string, string) (*domain.Record, error)
	Save(context.Context, domain.Record) (domain.Record, error)
	Event(context.Context, domain.Event) error
	Director(context.Context, string) error
	History(context.Context, string) ([]domain.Event, error)
}
type Context interface {
	Access(context.Context, domain.Actor, string, string) (domain.Access, error)
	Source(context.Context, string, string) (domain.Source, error)
}
type UnitOfWork interface {
	Within(context.Context, func(context.Context, Repository, Context) error) error
}
type Authorizer interface {
	Resolve(context.Context, string) (domain.Actor, error)
}
type Values interface {
	ID() string
	Now() domain.Timestamp
}
