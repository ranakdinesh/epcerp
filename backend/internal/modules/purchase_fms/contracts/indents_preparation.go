package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type IndentsPreparationSource = domain.IndentsSource
type IndentsError = domain.IndentsError

// Internal current eligibility projection: never an authorization grant.
type IndentsPreparationReader interface {
	Line(context.Context, string, string) (IndentsPreparationSource, error)
}
