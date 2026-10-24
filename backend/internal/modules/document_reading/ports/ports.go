package ports

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/document_reading/domain"
)

type Repository interface {
	Submit(context.Context, domain.Input, []byte, string) (domain.Document, error)
	Get(context.Context, string, string, string) (domain.Document, error)
	List(context.Context, string, string) ([]domain.Document, error)
	Result(context.Context, string) ([]byte, string, error)
	Retry(context.Context, string, int32, string) (domain.Document, error)
	Claim(context.Context, string) (*domain.Document, error)
	Finish(context.Context, domain.Document, []byte, string, domain.Result, string) (domain.Document, error)
	Event(context.Context, domain.Document, string, string) error
	History(context.Context, string) ([]domain.Event, error)
}
type Cipher interface {
	Seal([]byte) ([]byte, string, error)
	Open([]byte, string) ([]byte, error)
	Hash([]byte) string
}
type Values interface{ ID() string }
type Inspector interface{ Inspect([]byte, string) error }
type Engine interface {
	Read(context.Context, []byte, string, string) (domain.Result, error)
}
type Codec interface {
	Encode(domain.Result) ([]byte, error)
	Text([]byte) ([]byte, error)
}
type UnitOfWork interface {
	Within(context.Context, func(context.Context, Repository) error) error
}
