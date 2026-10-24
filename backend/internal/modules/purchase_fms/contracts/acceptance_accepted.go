package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type AcceptanceAccepted = domain.AcceptanceRecord
type AcceptanceError = domain.AcceptanceError

// Consumer owns its authorization. Current acceptance is tied to the same released
// order and excludes unresolved exceptions, blockers and inactive source records.
type AcceptanceAcceptedReader interface {
	Accepted(context.Context, string, string) (*AcceptanceAccepted, error)
}
