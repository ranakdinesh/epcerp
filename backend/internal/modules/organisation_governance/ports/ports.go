package ports

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/domain"
)

type Repository interface {
	Lock(context.Context) error
	Get(context.Context) (*domain.Designation, error)
	Save(context.Context, domain.Designation) (domain.Designation, error)
	Event(context.Context, domain.Event) error
	History(context.Context) ([]domain.Event, error)
}
type Users interface {
	Get(context.Context, string) (*domain.User, error)
	List(context.Context) ([]domain.User, error)
}
type UnitOfWork interface {
	Within(context.Context, func(context.Context, Repository, Users) error) error
}
type Authorizer interface {
	Resolve(context.Context, string) (domain.Actor, error)
}
type Values interface {
	ID() string
	Now() domain.Timestamp
}
