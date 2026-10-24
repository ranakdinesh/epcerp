package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type WorkspaceCandidate = domain.WorkspaceCandidate

var WorkspaceErrIneligible = domain.WorkspaceIneligible
var WorkspaceErrAlreadyAssigned = domain.WorkspaceStale

// Ownership is transaction-bound; the caller authorizes project creation/assignment.
type WorkspaceOwnership interface {
	Candidates(context.Context) ([]WorkspaceCandidate, error)
	Initialize(context.Context, string, string, string) error
	Owner(context.Context, string) (string, error)
}
