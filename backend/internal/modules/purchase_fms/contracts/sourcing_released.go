package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type SourcingReleased = domain.SourcingReleased
type SourcingError = domain.SourcingError

// Reader is an internal, exact project/package, transaction-bound fact reader.
// Callers must authorize their own capability before consuming the LOI.
type SourcingReleasedReader interface {
	Released(context.Context, string, string) (*SourcingReleased, error)
}

// AuthorityReader returns only a currently valid exact released LOI. A historical
// released fact alone is insufficient to authorize a new financial commitment.
type SourcingAuthorityReader interface {
	CurrentReleased(context.Context, string, string) (*SourcingReleased, error)
}
