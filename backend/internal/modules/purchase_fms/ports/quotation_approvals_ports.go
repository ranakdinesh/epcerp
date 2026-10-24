package ports

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type QuotationApprovalsRepository interface {
	Get(context.Context, string, string) (*domain.QuotationApprovalsRecord, error)
	Save(context.Context, domain.QuotationApprovalsRecord, string) (domain.QuotationApprovalsRecord, error)
	History(context.Context, string, string) ([]domain.QuotationApprovalsEvent, error)
	Event(context.Context, domain.QuotationApprovalsEvent) error
}
type QuotationApprovalsContext interface {
	Actions(context.Context, domain.Principal, string) (domain.QuotationApprovalsActions, error)
	Source(context.Context, string, string) (domain.QuotationApprovalsSource, error)
}
type QuotationApprovalsFinance interface {
	Decide(context.Context, domain.Principal, domain.QuotationApprovalsRecord, domain.QuotationApprovalsAttempt, string) (string, error)
}
type QuotationApprovalsUnitOfWork interface {
	Within(context.Context, func(context.Context, QuotationApprovalsRepository, QuotationApprovalsContext, QuotationApprovalsFinance) error) error
}
type QuotationApprovalsAuthorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}
type QuotationApprovalsValues interface {
	ID() string
	Now() domain.QuotationApprovalsTimestamp
}

// Queue reads only this capability's submitted sets; application filters each
// candidate with live project-scoped Accounts authority before publishing it.
type QuotationApprovalsQueueRepository interface {
	Queue(context.Context, int32) ([]domain.QuotationApprovalsRecord, error)
}
type QuotationApprovalsQueueContext interface {
	ProjectLabel(context.Context, string) (string, string, error)
}

type QuotationApprovalsPrinter interface {
	Render(context.Context, domain.QuotationApprovalsPrint) ([]byte, error)
}

type QuotationApprovalsWorkContext interface {
	Reviewers(context.Context, string) ([]domain.Principal, error)
}
type QuotationApprovalsWorkRepository interface {
	Notify(context.Context, domain.QuotationWorkPlan) error
}

type QuotationApprovalsProjectRepository interface {
	ProjectRecords(context.Context, string) ([]domain.QuotationApprovalsRecord, error)
}
type QuotationApprovalsProjectContext interface {
	ProjectRead(context.Context, domain.Principal, string) (bool, error)
}

// Optional draft storage participates in the owning approval transaction.
type QuotationShortlistDraftRepository interface {
	Drafts(context.Context, string, string) ([]domain.QuotationShortlistDraft, error)
	SaveDraft(context.Context, domain.QuotationShortlistDraft, string) (domain.QuotationShortlistDraft, error)
	CompleteDraft(context.Context, string, string, string, string) error
}
