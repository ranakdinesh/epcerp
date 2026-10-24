package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type QualityVerified = domain.QualityRecord
type QualityError = domain.QualityError

// Current required quality evidence, recalculated in the consuming transaction.
// Consumers own authorization and independent client/dispatch decisions.
type QualityVerifiedReader interface {
	Verified(context.Context, string, string) (*QualityVerified, error)
}
