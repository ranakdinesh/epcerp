package ports

import (
	"context"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type ReferenceReader interface {
	Get(context.Context, string, string) (*masterDomain.Reference, error)
}
type ReferenceRepository interface {
	ReferenceReader
	List(context.Context, string, string, bool) ([]masterDomain.Reference, error)
	Save(context.Context, string, masterDomain.Reference, bool) error
	Status(context.Context, string, string, string, bool) error
	RetireCities(context.Context, string, string) error
	LockMutations(context.Context) error
}
type ReferenceUnitOfWork interface {
	Within(context.Context, func(context.Context, ReferenceRepository) error) error
}

type ReferenceValues interface{ ID() string }
