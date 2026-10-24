package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type HoldsEvaluation = domain.HoldsEvaluation
type HoldsEvaluator interface {
	Evaluate(context.Context, string, string, string) (HoldsEvaluation, error)
}
