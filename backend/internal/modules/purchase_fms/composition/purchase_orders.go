package composition

import (
	"github.com/jackc/pgx/v5/pgxpool"

	documentInfra "github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	masterModule "github.com/ranakdinesh/epcERP/internal/modules/master"
	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	purchaseHTTP "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	purchasePorts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

func registerPurchaseOrders(services *purchaseHTTP.Services, pool *pgxpool.Pool, a auth.Resolver, p purchaseRepository.IndentsFactory, j func(database.Connection) events.Writer, cipher *documentInfra.DocumentCipher) error {
	factory := func(c database.Connection) purchasePorts.OrdersContext {
		source := p(c)
		sp := purchaseSourcingProviders(c, source, cipher)
		tp := purchaseTechnicalProviders(c, source)
		tp.Clearances = purchaseRepository.NewQuotationProjectClearance(c, quotationApprovalProviders(c, cipher))
		suppliers := masterRepository.NewVendorDirectory(c)
		gate := masterRepository.NewVendorVerificationGate(c, suppliers, masterModule.DocumentEvidence(c, cipher, documentInfra.NewDocumentInspector(masterDomain.VendorDocumentMaximumBytes)))
		return purchaseRepository.NewOrdersContext(purchaseRepository.OrdersProviders{Projects: tp.Projects, Purchase: tp.Purchase, Access: tp.Access, Users: tp.Users, Work: tp.Work, Authority: purchaseRepository.NewSourcingAuthorityReader(c, sp), Technical: purchaseRepository.NewTechnicalClearanceReader(c, tp), Verification: gate, Verified: gate, Suppliers: suppliers, Indents: purchaseRepository.NewIndentsReader(c)})
	}
	objects, bucket, err := masterModule.DocumentObjects()
	if err != nil {
		return err
	}
	services.Orders = purchaseApp.NewOrders(purchaseRepository.NewOrdersUnitOfWork(pool, factory, j), purchaseRepository.NewAuthBridge(a), purchaseRepository.Values{}).WithEvidence(cipher, purchaseRepository.TechnicalInspector{}, objects, bucket)
	return nil
}
