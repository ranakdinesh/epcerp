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

func purchaseSiteReceiptProviders(c database.Connection, p purchaseRepository.IndentsProviders) purchaseRepository.SiteReceiptsProviders {
	x := purchaseDispatchProviders(c, p)
	return purchaseRepository.SiteReceiptsProviders{Projects: x.Projects, Purchase: x.Purchase, Access: x.Access, Users: x.Users, Work: x.Work, Dispatch: purchaseRepository.NewDispatchDispatchedReader(c, x)}
}
func registerPurchaseSiteReceipts(services *purchaseHTTP.Services, pool *pgxpool.Pool, a auth.Resolver, p purchaseRepository.IndentsFactory, j func(database.Connection) events.Writer) {
	factory := func(c database.Connection) purchasePorts.SiteReceiptsContext {
		return purchaseRepository.NewSiteReceiptsContext(purchaseSiteReceiptProviders(c, p(c)))
	}
	services.SiteReceipts = purchaseApp.NewSiteReceipts(purchaseRepository.NewSiteReceiptsUnitOfWork(pool, factory, j), purchaseRepository.NewAuthBridge(a), purchaseRepository.Values{})
}
