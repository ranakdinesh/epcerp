package ports

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type InspectionReportsRepository interface {
	Get(context.Context, string, string) (*domain.InspectionReportsRecord, error)
	Save(context.Context, domain.InspectionReportsRecord) (domain.InspectionReportsRecord, error)
	Event(context.Context, domain.InspectionReportsEvent) error
	History(context.Context, string) ([]domain.InspectionReportsEvent, error)
}
type InspectionReportsContext interface {
	Access(context.Context, domain.Principal, string, string) (domain.InspectionReportsAccess, error)
	Source(context.Context, string, string) (domain.InspectionReportsSource, error)
}
type InspectionReportsUnitOfWork interface {
	Within(context.Context, func(context.Context, InspectionReportsRepository, InspectionReportsContext) error) error
}
type InspectionReportsAuthorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}
type InspectionReportsValues interface {
	ID() string
	Now() domain.InspectionReportsTimestamp
}
