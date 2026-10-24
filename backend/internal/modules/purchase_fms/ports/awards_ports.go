package ports

import (
	"context"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type AwardsRepository interface {
	Lock(context.Context, string) error
	Get(context.Context, string, string) (*domain.AwardHandoff, error)
	Save(context.Context, domain.AwardHandoff, string) error
}
type AwardsContext interface {
	Authorize(context.Context, domain.Principal, string, bool) error
	Candidates(context.Context, string) ([]domain.AwardCandidate, error)
}
type AwardsSession struct {
	Documents       LOIDocumentsRepository
	DocumentContext LOIDocumentContext
	Awards          AwardsRepository
	Context         AwardsContext
	Indents         IndentsRepository
	IndentContext   IndentsContext
	Workflow        WorkflowRepository
	WorkContext     WorkflowContext
	Sourcing        SourcingRepository
	SourceContext   SourcingContext
}
type AwardsUnitOfWork interface {
	WithinAwards(context.Context, func(context.Context, AwardsSession) error) error
}
