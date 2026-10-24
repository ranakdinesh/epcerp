package ports

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type TechnicalRepository interface {
	TechnicalEvidenceRepository
	Get(context.Context, string, string) (*domain.TechnicalCase, error)
	Save(context.Context, domain.TechnicalCase) (domain.TechnicalCase, error)
	Files(context.Context, string) ([]domain.TechnicalFile, error)
	History(context.Context, string) ([]domain.TechnicalEvent, error)
	Event(context.Context, domain.TechnicalEvent) error
}
type TechnicalContext interface {
	Access(context.Context, domain.Principal, string, string) (domain.TechnicalAccess, error)
	Source(context.Context, string, string) (domain.TechnicalSource, error)
}
type TechnicalUnitOfWork interface {
	Within(context.Context, func(context.Context, TechnicalRepository, TechnicalContext) error) error
}
type TechnicalAuthorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}
type TechnicalValues interface {
	ID() string
	Now() domain.TechnicalTimestamp
}

type TechnicalEvidenceRepository interface {
	Evidence(context.Context, string, string) (*domain.TechnicalEvidence, error)
	StoreEvidence(context.Context, domain.TechnicalEvidence) error
	EvidenceEvent(context.Context, domain.TechnicalFileEvent) error
}
type TechnicalCipher interface {
	Seal([]byte) ([]byte, string, error)
	Open([]byte, string) ([]byte, error)
}
type TechnicalInspector interface {
	Inspect(string, string, []byte) error
}
