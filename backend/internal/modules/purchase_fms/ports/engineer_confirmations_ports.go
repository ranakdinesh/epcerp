package ports

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type EngineerConfirmationsRepository interface {
	Get(context.Context, string, string) (*domain.EngineerConfirmationsRecord, error)
	Save(context.Context, domain.EngineerConfirmationsRecord) (domain.EngineerConfirmationsRecord, error)
	Event(context.Context, domain.EngineerConfirmationsEvent) error
	History(context.Context, string) ([]domain.EngineerConfirmationsEvent, error)
}
type EngineerConfirmationsContext interface {
	Access(context.Context, domain.Principal, string, string) (domain.EngineerConfirmationsAccess, error)
	Source(context.Context, string, string) (domain.EngineerConfirmationsSource, error)
}
type EngineerConfirmationsUnitOfWork interface {
	Within(context.Context, func(context.Context, EngineerConfirmationsRepository, EngineerConfirmationsContext) error) error
}
type EngineerConfirmationsAuthorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}
type EngineerConfirmationsValues interface {
	ID() string
	Now() domain.EngineerConfirmationsTimestamp
}
