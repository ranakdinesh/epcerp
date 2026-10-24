package ports

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/domain"
)

type Repository interface {
	UpdateMetadata(context.Context, domain.Project, domain.Correction) (domain.Project, error)
	MetadataHistory(context.Context, string) ([]domain.MetadataChange, error)
	Get(context.Context, string, bool) (*domain.Project, error)
	List(context.Context, domain.Filter, string) (domain.Page, error)
	Create(context.Context, domain.Project) (domain.Project, error)
	Transfer(context.Context, domain.Project, string) (domain.Project, error)
	Assignment(context.Context, domain.Assignment) error
	History(context.Context, string) ([]domain.Assignment, error)
	RecalculationProjects(context.Context) ([]domain.Project, error)
	SaveCalculation(context.Context, domain.Project, string) error
}
type Clients interface {
	Get(context.Context, string) (*domain.Client, error)
	List(context.Context) ([]domain.Client, error)
}
type Users interface {
	Get(context.Context, string) (*domain.Candidate, error)
	List(context.Context) ([]domain.Candidate, error)
}
type Calendar interface {
	Count(context.Context, string, string) (domain.WorkingCount, error)
}
type Purchasing interface {
	Candidates(context.Context) ([]domain.Candidate, error)
	Initialize(context.Context, string, string, string) error
	Owner(context.Context, string) (string, error)
}
type UnitOfWork interface {
	Within(context.Context, func(context.Context, Repository, Clients, Users, Calendar, Purchasing) error) error
}
type Authorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}
type Values interface{ ID() string }

// Specialist metadata scope is supplied through a published live assignment port.
type ScopedReadRepository interface {
	ListReadable(context.Context, domain.Filter, domain.Principal) (domain.Page, error)
	CanReadAssigned(context.Context, domain.Principal, string) (bool, error)
}
