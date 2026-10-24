package ports

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type SiteReceiptsRepository interface {
	Get(context.Context, string, string) (*domain.SiteReceiptsRecord, error)
	Save(context.Context, domain.SiteReceiptsRecord) (domain.SiteReceiptsRecord, error)
	Event(context.Context, domain.SiteReceiptsEvent) error
	History(context.Context, string) ([]domain.SiteReceiptsEvent, error)
}
type SiteReceiptsContext interface {
	Access(context.Context, domain.Principal, string, string) (domain.SiteReceiptsAccess, error)
	Source(context.Context, string, string) (domain.SiteReceiptsSource, error)
}
type SiteReceiptsUnitOfWork interface {
	Within(context.Context, func(context.Context, SiteReceiptsRepository, SiteReceiptsContext) error) error
}
type SiteReceiptsAuthorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}
type SiteReceiptsValues interface {
	ID() string
	Now() domain.SiteReceiptsTimestamp
}
