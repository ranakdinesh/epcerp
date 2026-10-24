package ports

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/storage/domain"
)

type Repository interface {
	Save(context.Context, domain.Document) (domain.Document, error)
	Get(context.Context, string, bool) (*domain.Document, error)
	List(context.Context, string, string) ([]domain.Document, error)
	Deactivate(context.Context, string, string) (*domain.Document, error)
	Event(context.Context, domain.Event) error
}
type UnitOfWork interface {
	Within(context.Context, func(context.Context, Repository) error) error
}
type Authorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}
type Objects interface {
	Put(context.Context, string, string, []byte) error
	Get(context.Context, string, string) ([]byte, error)
}
type Encryptor interface {
	Encrypt([]byte) ([]byte, error)
	Decrypt([]byte) ([]byte, error)
	Hash([]byte) string
	Algorithm() string
}
type Values interface {
	ID() string
	Now() int64
}
