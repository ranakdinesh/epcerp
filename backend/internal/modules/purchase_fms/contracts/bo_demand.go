package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type BODemandLink = domain.BODemandLink
type BOError = domain.BOError

// DemandReader maps current active completed BO evidence by exact project/demand identity.
// A nil result is missing current source, never an approval. The consuming transaction and its authorization remain mandatory.
type BODemandReader interface {
	Demand(context.Context, string, string) (*BODemandLink, error)
}
