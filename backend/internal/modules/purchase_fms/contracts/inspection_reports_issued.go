package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type InspectionReportsIssued = domain.InspectionReportsRecord
type InspectionReportsError = domain.InspectionReportsError

// Issued is recalculated against current inspection on the consuming transaction.
// Consumer authorization and later dispatch eligibility are separate rules.
type InspectionReportsIssuedReader interface {
	Issued(context.Context, string, string) (*InspectionReportsIssued, error)
}
