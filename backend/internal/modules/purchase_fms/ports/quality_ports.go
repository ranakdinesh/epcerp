package ports

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type QualityRepository interface {
	Get(context.Context, string, string) (*domain.QualityRecord, error)
	Save(context.Context, domain.QualityRecord) (domain.QualityRecord, error)
	Event(context.Context, domain.QualityEvent) error
	History(context.Context, string) ([]domain.QualityEvent, error)
}
type QualityContext interface {
	Access(context.Context, domain.Principal, string, string) (domain.QualityAccess, error)
	Source(context.Context, string, string) (domain.QualitySource, error)
}
type QualityUnitOfWork interface {
	Within(context.Context, func(context.Context, QualityRepository, QualityContext) error) error
}
type QualityAuthorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}
type QualityValues interface {
	ID() string
	Now() domain.QualityTimestamp
}
