package ports

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type WorkflowRepository interface {
	Exists(context.Context, string, string, string) (bool, error)
	Get(context.Context, string, string) (*domain.WorkflowPackage, error)
	Save(context.Context, domain.WorkflowPackage, string) (domain.WorkflowPackage, error)
	History(context.Context, string, string) ([]domain.WorkflowEvent, error)
	Event(context.Context, domain.WorkflowEvent) error
}
type WorkflowContext interface {
	Actions(context.Context, domain.Principal, string) (domain.WorkflowAccess, error)
	Source(context.Context, string, string, string) (domain.WorkflowSource, error)
	Candidates(context.Context, string) ([]domain.WorkflowCandidate, error)
	Members(context.Context, string) ([]domain.WorkflowMember, error)
}
type WorkflowUnitOfWork interface {
	Within(context.Context, func(context.Context, WorkflowRepository, WorkflowContext) error) error
}
type WorkflowAuthorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}
type WorkflowValues interface {
	ID() string
	Now() domain.WorkflowTimestamp
}

type WorkflowPageRepository interface {
	Page(context.Context, string, domain.WorkflowPageFilter) ([]domain.WorkflowSummary, int64, error)
}
