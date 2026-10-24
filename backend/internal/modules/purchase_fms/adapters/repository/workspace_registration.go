package repository

import (
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	application "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
)

func NewWorkspaceOwnership(c database.Connection, users auth.AssignmentDirectory, events events.Writer) contracts.WorkspaceOwnership {
	return application.WorkspaceRegistration{Repo: &workspaceRepository{q: NewStore(c), e: events, c: c}, Users: &workspaceUserBridge{users}, Values: Values{}}
}
