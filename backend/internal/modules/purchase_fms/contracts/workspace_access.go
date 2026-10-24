package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type WorkspacePrincipal = domain.Principal
type WorkspaceAccess = domain.WorkspaceAccess

// Internal scoped gate. Permission comes from the consumer's declared capability,
// actor from Auth; transaction lock prevents concurrent ownership handovers.
type WorkspaceAccessGate interface {
	Check(context.Context, WorkspacePrincipal, string, string, bool) (WorkspaceAccess, error)
}

type WorkspaceInvalid = domain.WorkspaceInvalid

var WorkspaceErrForbidden = domain.WorkspaceForbidden
var WorkspaceErrUnauthorized = domain.WorkspaceUnauthorized
var WorkspaceErrMissing = domain.WorkspaceMissing

// HoldGate permits recording/resolving holds on an on-hold project while preserving scope and closed-project guards.
type WorkspaceHoldGate interface {
	CheckHold(context.Context, WorkspacePrincipal, string, string, bool) (WorkspaceAccess, error)
}
