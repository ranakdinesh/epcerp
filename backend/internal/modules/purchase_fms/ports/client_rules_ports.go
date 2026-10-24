package ports

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type ClientRulesRepository interface {
	Lock(context.Context) error
	List(context.Context, string) ([]domain.ClientRulesRecord, error)
	Get(context.Context, string, string) (*domain.ClientRulesRecord, error)
	Save(context.Context, domain.ClientRulesRecord, string) (domain.ClientRulesRecord, error)
	Event(context.Context, domain.ClientRulesEvent) error
	History(context.Context, string, string) ([]domain.ClientRulesEvent, error)
}
type ClientRulesContext interface {
	Validate(context.Context, string, domain.ClientRulesData) error
}
type ClientRulesUnitOfWork interface {
	Within(context.Context, func(context.Context, ClientRulesRepository, ClientRulesContext) error) error
}
type ClientRulesAuthorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}
type ClientRulesValues interface{ ID() string }
