package ports

import (
	"context"
	domain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type DocumentApprovalLifecycle interface {
	Submitted(context.Context, domain.VendorDocumentDocument, string) error
	Reconciled(context.Context, domain.VendorDocumentDocument, string) error
	Decided(context.Context, domain.VendorVerificationSnapshot, []domain.VendorVerificationDecision, string) error
	Retired(context.Context, domain.VendorDocumentDocument, string, string) error
}
type DocumentApprovalRepository interface {
	Get(context.Context, string) (*domain.DocumentApproval, error)
	Save(context.Context, domain.DocumentApproval, string, string) error
	Reviewer(context.Context, string) (string, error)
	Task(context.Context, string, string, string, string, string, string) error
	Complete(context.Context, string, string, string, string, string) error
	Document(context.Context, string, string) (*domain.VendorDocumentDocument, error)
}
