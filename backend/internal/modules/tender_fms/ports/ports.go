package ports

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
)

type Repository interface {
	OwnerActive(context.Context, string) (bool, error)
	Lock(context.Context) error
	List(context.Context, string) ([]domain.Intake, error)
	Get(context.Context, string) (domain.Intake, error)
	FindInitial(context.Context, string, string) (*domain.Intake, error)
	Create(context.Context, string, string, string, string) (domain.Intake, error)
	Changed(context.Context, domain.Intake, string, string, string) error
	History(context.Context, string) ([]domain.Event, error)
}
type Documents interface {
	Validate(string, []byte) (string, string, error)
	Submit(context.Context, string, string, string, []byte) (domain.Source, error)
	List(context.Context, string) ([]domain.Source, error)
	Get(context.Context, string, string) (domain.Source, error)
	Artifact(context.Context, string, string, string) (domain.Artifact, error)
	Retry(context.Context, string, string, int32, string) (domain.Source, error)
	History(context.Context, string, string) ([]domain.Event, error)
}
type UnitOfWork interface {
	Within(context.Context, func(context.Context, Repository, Documents) error) error
}
type Authorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}
type Values interface{ ID() string }
