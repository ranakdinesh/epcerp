package ports

import (
	"context"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type LOIDocumentsRepository interface {
	List(context.Context, string, string) ([]domain.LOIDocument, error)
	Get(context.Context, string, string, string) (*domain.LOIDocument, error)
	Save(context.Context, domain.LOIDocument) error
	Event(context.Context, domain.LOIDocument, string, string) error
	Review(context.Context, string) (*domain.LOIReview, error)
	CompleteReview(context.Context, string, domain.LOIReview) error
}
type LOIDocumentContext interface {
	Metadata(context.Context, string, string) (domain.LOIContent, error)
	Specifications(context.Context, string) (string, error)
}
type LOIPrinter interface {
	Render(context.Context, domain.LOIContent, int32) ([]byte, error)
}
type LOIFingerprinter interface {
	Approval(domain.QuotationApprovalsAttempt) (string, error)
	Content(domain.LOIContent) (string, error)
}
