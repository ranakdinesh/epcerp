package composition

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authout "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authapp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	accessmodule "github.com/ranakdinesh/epcERP/internal/modules/project_access"
	accessout "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound"
	purchaseHTTP "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	purchasePorts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

func registerPurchaseAcceptance(services *purchaseHTTP.Services, pool *pgxpool.Pool, a auth.Resolver, p purchaseRepository.IndentsFactory, j func(database.Connection) events.Writer) {
	factory := func(c database.Connection) purchasePorts.AcceptanceContext {
		source := p(c)
		return purchaseRepository.NewAcceptanceContext(purchaseRepository.AcceptanceProviders{Projects: source.Projects, Purchase: source.Scope, Access: accessout.NewGate(c, accessmodule.Providers(c)), Users: authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c)), Work: purchaseRepository.NewWorkflowSourceReader(c, purchaseWorkflowProviders(c, source)), Orders: purchaseRepository.NewOrdersReleasedReader(c)})
	}
	services.Acceptance = purchaseApp.NewAcceptance(purchaseRepository.NewAcceptanceUnitOfWork(pool, factory, j), purchaseRepository.NewAuthBridge(a), purchaseRepository.Values{})
}
