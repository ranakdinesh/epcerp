package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type ClientClearanceCleared = domain.ClientClearanceRecord
type ClientClearanceError = domain.ClientClearanceError

// Current client MDCC; consumer owns authorization and shipment decisions.
type ClientClearanceClearedReader interface {
	Cleared(context.Context, string, string) (*ClientClearanceCleared, error)
}
