package ports

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/work_calendar/domain"
)

type Principal struct {
	ID                string
	CanRead, CanWrite bool
}
type Auth interface {
	Resolve(context.Context, string) (Principal, error)
}
type Repository interface {
	List(context.Context, int) ([]domain.Holiday, error)
	Dates(context.Context) (map[string]bool, error)
	Save(context.Context, domain.Change, string) (domain.Holiday, error)
}
type Unit interface {
	Within(context.Context, func(context.Context, Repository, func(context.Context, string) (int, error)) error) error
}
