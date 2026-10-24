package contracts

import (
	"context"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

type AssignmentCandidate = domain.AssignmentCandidate

// AssignmentDirectory is internal, bound to the consumer transaction by composition.
type AssignmentDirectory interface {
	Get(context.Context, string) (*AssignmentCandidate, error)
	List(context.Context) ([]AssignmentCandidate, error)
}
