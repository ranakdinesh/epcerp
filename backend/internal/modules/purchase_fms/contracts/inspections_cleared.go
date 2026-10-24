package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type InspectionsCleared = domain.InspectionsRecord
type InspectionsError = domain.InspectionsError

// Consumer owns authorization; current clearance is revalidated on its shared transaction.
type InspectionsClearedReader interface {
	Cleared(context.Context, string, string) (*InspectionsCleared, error)
}
