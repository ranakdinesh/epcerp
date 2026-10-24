package composition

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	accessmodule "github.com/ranakdinesh/epcERP/internal/modules/project_access"
	accessout "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound"
	purchaseHTTP "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
)

func purchaseWorkflowProviders(c database.Connection, p purchaseRepository.IndentsProviders) purchaseRepository.WorkflowProviders {
	return purchaseRepository.WorkflowProviders{Projects: p.Projects, Purchase: p.Scope, Access: accessout.NewGate(c, accessmodule.Providers(c)), Users: p.Users, Indents: purchaseRepository.NewIndentsReader(c), Preparation: purchaseRepository.NewIndentsPreparationReader(c, p)}
}
func registerPurchaseWorkflow(services *purchaseHTTP.Services, pool *pgxpool.Pool, a auth.Resolver, p purchaseRepository.IndentsFactory, j func(database.Connection) events.Writer) {
	services.Workflow = purchaseApp.NewWorkflow(purchaseRepository.NewWorkflowUnitOfWork(pool, func(c database.Connection) purchaseRepository.WorkflowProviders {
		return purchaseWorkflowProviders(c, p(c))
	}, j), purchaseRepository.NewAuthBridge(a), purchaseRepository.Values{})
}
