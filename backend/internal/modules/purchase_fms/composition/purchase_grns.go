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

func purchaseGRNProviders(c database.Connection, p purchaseRepository.IndentsProviders) purchaseRepository.GRNsProviders {
	x := purchaseSiteReceiptProviders(c, p)
	return purchaseRepository.GRNsProviders{Projects: x.Projects, Purchase: x.Purchase, Access: x.Access, Users: x.Users, Work: x.Work, Receipt: purchaseRepository.NewSiteReceiptsDeliveredReader(c, x)}
}
func registerPurchaseGRNs(services *purchaseHTTP.Services, pool *pgxpool.Pool, a auth.Resolver, p purchaseRepository.IndentsFactory, j func(database.Connection) events.Writer) {
	factory := func(c database.Connection) purchasePorts.GRNsContext {
		return purchaseRepository.NewGRNsContext(purchaseGRNProviders(c, p(c)))
	}
	services.GRNs = purchaseApp.NewGRNs(purchaseRepository.NewGRNsUnitOfWork(pool, factory, j), purchaseRepository.NewAuthBridge(a), purchaseRepository.Values{})
}
