package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type GRNsIssued = domain.GRNsRecord
type GRNsError = domain.GRNsError

// Current issued inwarding evidence; consumer authorizes engineer/accounting workflow.
type GRNsIssuedReader interface {
	Issued(context.Context, string, string) (*GRNsIssued, error)
}
