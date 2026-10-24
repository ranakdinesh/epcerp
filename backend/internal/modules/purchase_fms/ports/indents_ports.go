package ports

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type IndentsRepository interface {
	List(context.Context, string) ([]domain.IndentsRecord, error)
	Get(context.Context, string, string) (*domain.IndentsRecord, error)
	Save(context.Context, domain.IndentsRecord, string) (domain.IndentsRecord, error)
	Event(context.Context, domain.IndentsEvent) error
	History(context.Context, string, string) ([]domain.IndentsEvent, error)
}
type IndentsContext interface {
	Scope(context.Context, domain.Principal, string, string, bool) error
	Validate(context.Context, string, domain.IndentsInput) error
	Line(context.Context, string, string) (domain.IndentsSource, error)
	WorkspaceSource(context.Context, string, bool, []domain.IndentsRecord) (domain.IndentsWorkspaceSource, error)
}
type IndentsUnitOfWork interface {
	Within(context.Context, func(context.Context, IndentsRepository, IndentsContext) error) error
}
type IndentsAuthorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}
type IndentsValues interface {
	ID() string
	Now() domain.IndentsTimestamp
}
