package ports

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type ProgressAuthorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}

// The reader authorizes actual project scope and live grants before reading any
// commercial source. Composition must bind every owner to the same transaction.
type ProgressReader interface {
	Authorize(context.Context, domain.Principal, string) error
	Source(context.Context, string) (domain.ProgressSource, error)
}
type ProgressUnitOfWork interface {
	Within(context.Context, func(context.Context, ProgressReader) error) error
}
type ProgressValues interface {
	Now() domain.ProgressTimestamp
}

// A composition supplies current published owner facts on the consumer transaction.
// There is deliberately no HTTP model or command accepting these source facts.
type ProgressSourceReader interface {
	Read(context.Context, string) (domain.ProgressSource, error)
}
