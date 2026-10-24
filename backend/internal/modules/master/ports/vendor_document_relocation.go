package ports

import (
	"context"
	domain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

// Operator-only relocation changes physical storage without changing business evidence.
type VendorDocumentRelocationRepository interface {
	NextInline(context.Context) (*domain.VendorDocumentDocument, error)
	Relocate(context.Context, domain.VendorDocumentDocument, string, string, string) error
}
type VendorDocumentRelocationUnitOfWork interface {
	Within(context.Context, func(context.Context, VendorDocumentRelocationRepository) error) error
}
