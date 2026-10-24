package composition

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	purchaseHTTP "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	purchasePorts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

func registerPurchaseInspections(services *purchaseHTTP.Services, pool *pgxpool.Pool, a auth.Resolver, p purchaseRepository.IndentsFactory, j func(database.Connection) events.Writer) {
	factory := func(c database.Connection) purchasePorts.InspectionsContext {
		return purchaseRepository.NewInspectionsContext(purchaseInspectionProviders(c, p(c)))
	}
	services.Inspections = purchaseApp.NewInspections(purchaseRepository.NewInspectionsUnitOfWork(pool, factory, j), purchaseRepository.NewAuthBridge(a), purchaseRepository.Values{})
}

func purchaseInspectionProviders(c database.Connection, source purchaseRepository.IndentsProviders) purchaseRepository.InspectionsProviders {
	technical := purchaseTechnicalProviders(c, source)
	orders := purchaseRepository.NewOrdersReleasedReader(c)
	accepted := purchaseRepository.NewAcceptanceAcceptedReader(c, purchaseRepository.AcceptanceProviders{Projects: source.Projects, Purchase: source.Scope, Access: technical.Access, Users: source.Users, Work: technical.Work, Orders: orders})
	return purchaseRepository.InspectionsProviders{Projects: source.Projects, Purchase: source.Scope, Access: technical.Access, Users: source.Users, Work: technical.Work, Orders: orders, Accepted: accepted, Technical: purchaseRepository.NewTechnicalClearanceReader(c, technical)}
}
