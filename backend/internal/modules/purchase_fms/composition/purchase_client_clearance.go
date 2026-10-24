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

func registerPurchaseClientClearance(services *purchaseHTTP.Services, pool *pgxpool.Pool, a auth.Resolver, p purchaseRepository.IndentsFactory, j func(database.Connection) events.Writer) {
	factory := func(c database.Connection) purchasePorts.ClientClearanceContext {
		return purchaseRepository.NewClientClearanceContext(purchaseClientClearanceProviders(c, p(c)))
	}
	services.ClientClearance = purchaseApp.NewClientClearance(purchaseRepository.NewClientClearanceUnitOfWork(pool, factory, j), purchaseRepository.NewAuthBridge(a), purchaseRepository.Values{})
}

func purchaseClientClearanceProviders(c database.Connection, x purchaseRepository.IndentsProviders) purchaseRepository.ClientClearanceProviders {
	q := purchaseQualityProviders(c, x)
	return purchaseRepository.ClientClearanceProviders{Projects: q.Projects, Purchase: q.Purchase, Access: q.Access, Users: q.Users, Work: q.Work, Quality: purchaseRepository.NewQualityVerifiedReader(c, q)}
}
