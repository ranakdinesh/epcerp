package contracts

import (
	"context"
)

type VendorDocumentEvidence struct {
	ID, DocType, SHA256       string
	Version                   int32
	Previewable               bool
	ApprovalKnown             bool
	Approval, ApprovalComment string
}
type VendorDocumentReader interface {
	Active(context.Context, string) ([]VendorDocumentEvidence, error)
}
