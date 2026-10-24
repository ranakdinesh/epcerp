package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type IndentDocumentsDocument = domain.IndentDocumentsDocument
type IndentDocumentsNotice = domain.IndentDocumentsNotice

// Reader is internal and bound to the caller transaction. Consumers must enforce
// their own report scope; it does not expose document commands or persistence.
type IndentDocumentsReader interface {
	List(context.Context, string, string) ([]IndentDocumentsDocument, error)
}

// Pending dispatch notices remain owned here and can be read by the central
// personal inbox with its authenticated recipient, without broad project access.
type IndentDocumentsNoticeReader interface {
	Notices(context.Context, string) ([]IndentDocumentsNotice, error)
}
