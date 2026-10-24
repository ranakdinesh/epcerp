package ports

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/domain"
)

type Repository interface {
	Source(context.Context, string) (*domain.Source, error)
	BySource(context.Context, string) (*domain.Notice, error)
	Create(context.Context, domain.Notice) (domain.Notice, error)
	Get(context.Context, string, string) (*domain.Notice, error)
	List(context.Context, string, domain.Query) ([]domain.Notice, int64, error)
	Save(context.Context, domain.Notice) (domain.Notice, error)
	Event(context.Context, domain.Event) error
	History(context.Context, string, string) ([]domain.Event, error)
}
type DirectorDirectory interface {
	Current(context.Context) (*domain.Director, error)
}
type Users interface {
	Get(context.Context, string) (*domain.Actor, error)
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

type TenderSources interface {
	Read(context.Context, string) (domain.TenderAction, error)
}
