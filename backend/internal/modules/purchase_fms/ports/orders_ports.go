package ports

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type OrdersRepository interface {
	Get(context.Context, string, string) (*domain.OrdersOrder, error)
	Save(context.Context, domain.OrdersOrder) (domain.OrdersOrder, error)
	Event(context.Context, domain.OrdersEvent) error
	History(context.Context, string) ([]domain.OrdersEvent, error)
	NextNumber(context.Context, int) (int64, error)
	DocumentEvent(context.Context, domain.OrdersDocumentEvent) error
}
type OrdersContext interface {
	Access(context.Context, domain.Principal, string, string) (domain.OrdersAccess, error)
	Source(context.Context, string, string) (domain.OrdersSource, error)
}
type OrdersUnitOfWork interface {
	Within(context.Context, func(context.Context, OrdersRepository, OrdersContext) error) error
}
type OrdersAuthorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}
type OrdersValues interface {
	ID() string
	Now() domain.OrdersTimestamp
}

type OrdersObjects interface {
	Put(context.Context, string, string, []byte) error
	Get(context.Context, string, string) ([]byte, error)
}
type OrdersEvidenceRepository interface {
	File(context.Context, string, string) (*domain.OrdersFile, error)
	StoreFile(context.Context, domain.OrdersFile) error
	FileEvent(context.Context, domain.OrdersFileEvent) error
}
