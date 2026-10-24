package ports

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type QuotationsRepository interface {
	Get(context.Context, string, string) (*domain.QuotationsRecord, error)
	Save(context.Context, domain.QuotationsRecord, string) (domain.QuotationsRecord, error)
	Event(context.Context, domain.QuotationsEvent) error
	History(context.Context, string, string) ([]domain.QuotationsEvent, error)
	Revisions(context.Context, string, string) ([]domain.QuotationsQuote, error)
	Attachments(context.Context, string) ([]domain.QuotationsAttachment, error)
	Attachment(context.Context, string, string) (*domain.QuotationsAttachment, error)
	InsertAttachment(context.Context, domain.QuotationsAttachment) (domain.QuotationsAttachment, error)
	AttachmentEvent(context.Context, domain.QuotationsAttachment, string, string, string) error
}
type QuotationsContext interface {
	BOChoices(context.Context, string) ([]domain.QuotationsBOChoice, error)
	Scope(context.Context, domain.Principal, string, string, bool) (bool, error)
	Source(context.Context, string, string) (domain.QuotationsScope, error)
}
type QuotationsUnitOfWork interface {
	Within(context.Context, func(context.Context, QuotationsRepository, QuotationsContext) error) error
}
type QuotationsAuthorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}
type QuotationsApprovalGroups interface {
	Current(context.Context, string, string) ([]domain.QuotationsApprovalGroup, error)
}
type QuotationsValues interface {
	ID() string
	Now() domain.QuotationsTimestamp
}
type QuotationsCipher interface {
	Seal([]byte) ([]byte, string, error)
	Open([]byte, string) ([]byte, error)
}
type QuotationsInspector interface {
	Inspect(string, string, []byte) error
}

type QuotationsFreezeGuard interface {
	PendingDocument(context.Context, string, string, string) (bool, error)
}
