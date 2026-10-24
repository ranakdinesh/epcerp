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

func registerPurchaseInspectionReports(services *purchaseHTTP.Services, pool *pgxpool.Pool, a auth.Resolver, p purchaseRepository.IndentsFactory, j func(database.Connection) events.Writer) {
	factory := func(c database.Connection) purchasePorts.InspectionReportsContext {
		return purchaseRepository.NewInspectionReportsContext(purchaseReportProviders(c, p(c)))
	}
	services.InspectionReports = purchaseApp.NewInspectionReports(purchaseRepository.NewInspectionReportsUnitOfWork(pool, factory, j), purchaseRepository.NewAuthBridge(a), purchaseRepository.Values{})
}

func purchaseReportProviders(c database.Connection, x purchaseRepository.IndentsProviders) purchaseRepository.InspectionReportsProviders {
	ip := purchaseInspectionProviders(c, x)
	return purchaseRepository.InspectionReportsProviders{Projects: x.Projects, Purchase: x.Scope, Access: ip.Access, Users: x.Users, Work: ip.Work, Inspection: purchaseRepository.NewInspectionsClearedReader(c, ip)}
}
