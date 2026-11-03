package contracts

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/document_reading/domain"
)

type PDFLayoutReader interface {
	Extract(context.Context, []byte) (domain.PDFTables, error)
}
