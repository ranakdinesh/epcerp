package composition

import (
	"github.com/jackc/pgx/v5/pgxpool"

	documentInfra "github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	accessmodule "github.com/ranakdinesh/epcERP/internal/modules/project_access"
	accessout "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound"
	purchaseHTTP "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	purchasePorts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

func registerPurchaseSourcing(services *purchaseHTTP.Services, pool *pgxpool.Pool, a auth.Resolver, p purchaseRepository.IndentsFactory, j func(database.Connection) events.Writer, cipher *documentInfra.DocumentCipher) {
	factory := func(c database.Connection) purchasePorts.SourcingContext {
		return purchaseRepository.NewSourcingContext(purchaseSourcingProviders(c, p(c), cipher))
	}
	services.Sourcing = purchaseApp.NewSourcing(purchaseRepository.NewSourcingUnitOfWork(pool, factory, j), purchaseRepository.NewAuthBridge(a), purchaseRepository.Values{})
}

func purchaseSourcingProviders(c database.Connection, source purchaseRepository.IndentsProviders, cipher *documentInfra.DocumentCipher) purchaseRepository.SourcingProviders {
	return purchaseRepository.SourcingProviders{Projects: source.Projects, Purchase: source.Scope, Access: accessout.NewGate(c, accessmodule.Providers(c)), Users: source.Users, Work: purchaseRepository.NewWorkflowSourceReader(c, purchaseWorkflowProviders(c, source)), Indents: purchaseRepository.NewIndentsReader(c), Items: source.Items, Suppliers: masterRepository.NewVendorDirectory(c), BO: purchaseRepository.NewBODemandReader(c), Quotes: purchaseRepository.NewQuotationApprovalsApprovedReader(c, quotationApprovalProviders(c, cipher))}
}
