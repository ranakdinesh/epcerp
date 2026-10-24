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

func registerPurchaseQuality(services *purchaseHTTP.Services, pool *pgxpool.Pool, a auth.Resolver, p purchaseRepository.IndentsFactory, j func(database.Connection) events.Writer) {
	factory := func(c database.Connection) purchasePorts.QualityContext {
		return purchaseRepository.NewQualityContext(purchaseQualityProviders(c, p(c)))
	}
	services.Quality = purchaseApp.NewQuality(purchaseRepository.NewQualityUnitOfWork(pool, factory, j), purchaseRepository.NewAuthBridge(a), purchaseRepository.Values{})
}

func purchaseQualityProviders(c database.Connection, x purchaseRepository.IndentsProviders) purchaseRepository.QualityProviders {
	rp := purchaseReportProviders(c, x)
	return purchaseRepository.QualityProviders{Projects: rp.Projects, Purchase: rp.Purchase, Access: rp.Access, Users: rp.Users, Work: rp.Work, Reports: purchaseRepository.NewInspectionReportsIssuedReader(c, rp)}
}
