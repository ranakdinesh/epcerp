package integration

import (
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authOut "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authApp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	eventOut "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventApp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseContracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
)

func projectPurchaseOwnership(c database.Connection) purchaseContracts.WorkspaceOwnership {
	return purchaseRepository.NewWorkspaceOwnership(c, authApp.NewAssignmentDirectory(authOut.NewAssignmentReader(c)), eventApp.NewRecorder(eventOut.NewJournal(c)))
}
