package ports

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type WorkspaceRepository interface {
	Get(context.Context, string) (domain.Workspace, error)
	RegisterStates(context.Context, string) ([]domain.RegisterState, error)
	Assigned(context.Context, string) ([]string, error)
	Save(context.Context, string, domain.WorkspaceChange, string) (domain.Workspace, error)
	Record(context.Context, domain.WorkspaceHistory) error
	History(context.Context, string) ([]domain.WorkspaceHistory, error)
}
type WorkspaceProjects interface {
	Get(context.Context, string, bool) (*domain.WorkspaceProject, error)
	Page(context.Context, domain.WorkspaceFilter, string, []string, bool) ([]domain.WorkspaceProject, int64, error)
}
type WorkspaceUsers interface {
	Get(context.Context, string) (*domain.WorkspaceCandidate, error)
	List(context.Context) ([]domain.WorkspaceCandidate, error)
}
type WorkspaceUnitOfWork interface {
	Within(context.Context, func(context.Context, WorkspaceRepository, WorkspaceProjects, WorkspaceUsers) error) error
}
type WorkspaceAuthorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}
type WorkspaceValues interface{ ID() string }
