package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type AcceptanceProgress = domain.AcceptanceProgress

// Read-only reporting, not an AcceptedReader. Caller authorizes project scope.
// Holds preserve recorded acknowledgement; unresolved exceptions confer no readiness.
type AcceptanceProgressReader interface {
	Progress(context.Context, string, string) (AcceptanceProgress, error)
}
