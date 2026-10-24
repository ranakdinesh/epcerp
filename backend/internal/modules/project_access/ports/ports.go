package ports

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/project_access/domain"
)

type Repository interface {
	List(context.Context, string) ([]domain.Profile, error)
	Get(context.Context, string, string) (*domain.Profile, error)
	Save(context.Context, domain.Profile, string) (domain.Profile, error)
	Event(context.Context, domain.Event) error
	History(context.Context, string, string) ([]domain.Event, error)
}
type Context interface {
	Project(context.Context, string, bool) (*domain.Project, error)
	User(context.Context, string) (*domain.User, error)
	Bundle(context.Context, string) (*domain.Bundle, error)
	Member(context.Context, string, string) (bool, error)
	Options(context.Context) (domain.Options, error)
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
