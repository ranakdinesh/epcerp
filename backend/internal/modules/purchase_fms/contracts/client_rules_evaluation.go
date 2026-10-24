package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type ClientRulesEvaluation = domain.ClientRulesEvaluation
type ClientRulesVerdict = domain.ClientRulesVerdict

// Internal source gate bound to the consumer's shared transaction.
type ClientRulesEvaluator interface {
	Evaluate(context.Context, ClientRulesEvaluation) (ClientRulesVerdict, error)
}
