package ports

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/project_items/domain"
)

type Repository interface {
	List(context.Context, string) ([]domain.Record, error)
	Get(context.Context, string, string) (*domain.Record, error)
	Save(context.Context, domain.Record, string) (domain.Record, error)
	Event(context.Context, domain.Event) error
	History(context.Context, string, string) ([]domain.Event, error)
}
type Context interface {
	Catalogue(context.Context, string) ([]domain.CatalogueChoice, error)
	Scope(context.Context, domain.Actor, string, string, bool) error
	Item(context.Context, string, bool) (domain.Snapshot, error)
	Policy(context.Context, string, domain.Snapshot) error
	Committed(context.Context, string, string) (string, error)
}
type UnitOfWork interface {
	Within(context.Context, func(context.Context, Repository, Context) error) error
}
type Authorizer interface {
	Resolve(context.Context, string) (domain.Actor, error)
}
type Values interface{ ID() string }
