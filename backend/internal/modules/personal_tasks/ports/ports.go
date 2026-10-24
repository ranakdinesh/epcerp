package ports

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/personal_tasks/domain"
)

type Authorizer interface {
	Resolve(context.Context, string) (domain.Actor, error)
}
type Reader interface {
	Authorize(context.Context, domain.Actor) error
	Assigned(context.Context, domain.Actor, domain.Query) ([]domain.Task, int64, error)
}
type UnitOfWork interface {
	Within(context.Context, func(context.Context, Reader) error) error
}
type Values interface{ Now() domain.Timestamp }

type QuotationReader interface {
	Quotations(context.Context, domain.Actor) ([]domain.QuotationTask, error)
}

type SupplierReviewReader interface {
	SupplierReviews(context.Context, domain.Actor) ([]domain.SupplierReviewTask, error)
}
