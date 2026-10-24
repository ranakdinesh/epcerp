package ports

import (
	"context"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type VendorDocumentRepository interface {
	List(context.Context, string) ([]masterDomain.VendorDocumentDocument, error)
	Get(context.Context, string, string) (*masterDomain.VendorDocumentDocument, error)
	BySource(context.Context, string) (*masterDomain.VendorDocumentDocument, error)
	Insert(context.Context, masterDomain.VendorDocumentDocument) (*masterDomain.VendorDocumentDocument, error)
	Imported(context.Context, string, string, string) error
	Archive(context.Context, string, string, int32, string) (*masterDomain.VendorDocumentDocument, error)
	Changed(context.Context, masterDomain.VendorDocumentDocument, string, string, string) error
}
type VendorDocumentSuppliers interface {
	Exists(context.Context, string) (bool, error)
}
type VendorDocumentUnitOfWork interface {
	Within(context.Context, func(context.Context, VendorDocumentRepository, VendorDocumentSuppliers) error) error
}

type VendorDocumentCipher interface {
	Seal([]byte) ([]byte, string, error)
	Open([]byte, string) ([]byte, error)
	Hash([]byte) string
}
type VendorDocumentValues interface{ ID() string }

type VendorDocumentInspector interface{ Inspect([]byte) (string, error) }

// VendorDocumentObjects stores encrypted bytes only; references stay in the owning record.
type VendorDocumentObjects interface {
	Put(context.Context, string, string, []byte) error
	Get(context.Context, string, string) ([]byte, error)
}
