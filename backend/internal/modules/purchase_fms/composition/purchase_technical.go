package composition

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	documentInfra "github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	masterModule "github.com/ranakdinesh/epcERP/internal/modules/master"
	accessmodule "github.com/ranakdinesh/epcERP/internal/modules/project_access"
	accessout "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound"
	purchaseHTTP "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	purchasePorts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

func registerPurchaseTechnical(services *purchaseHTTP.Services, pool *pgxpool.Pool, a auth.Resolver, p purchaseRepository.IndentsFactory, j func(database.Connection) events.Writer, cipher *documentInfra.DocumentCipher) (func(http.Handler) http.Handler, error) {
	factory := func(c database.Connection) purchasePorts.TechnicalContext {
		tp := purchaseTechnicalProviders(c, p(c))
		tp.Clearances = purchaseRepository.NewQuotationProjectClearance(c, quotationApprovalProviders(c, cipher))
		return purchaseRepository.NewTechnicalContext(tp)
	}
	service := purchaseApp.NewTechnical(purchaseRepository.NewTechnicalUnitOfWork(pool, factory, j), purchaseRepository.NewAuthBridge(a), purchaseRepository.Values{})
	objects, bucket, err := masterModule.DocumentObjects()
	if err != nil {
		return nil, err
	}
	files := purchaseApp.NewTechnicalEvidence(service, cipher, purchaseRepository.TechnicalInspector{}).WithObjects(objects, bucket)
	services.Technical = service
	services.TechnicalEvidence = files
	return purchaseHTTP.TechnicalUploadGuard(files), nil
}

func purchaseTechnicalProviders(c database.Connection, source purchaseRepository.IndentsProviders) purchaseRepository.TechnicalProviders {
	return purchaseRepository.TechnicalProviders{Projects: source.Projects, Purchase: source.Scope, Access: accessout.NewGate(c, accessmodule.Providers(c)), Users: source.Users, Work: purchaseRepository.NewWorkflowSourceReader(c, purchaseWorkflowProviders(c, source)), Released: purchaseRepository.NewSourcingReleasedReader(c)}
}
