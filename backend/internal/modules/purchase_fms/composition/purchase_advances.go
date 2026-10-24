package composition

import (
	"github.com/jackc/pgx/v5/pgxpool"

	documentInfra "github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authout "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authapp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	inboxout "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/adapters/outbound"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	financeout "github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/adapters/outbound"
	masterModule "github.com/ranakdinesh/epcERP/internal/modules/master"
	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	govout "github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/adapters/outbound"
	accessmodule "github.com/ranakdinesh/epcERP/internal/modules/project_access"
	accessout "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound"
	purchaseHTTP "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	purchasePorts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

func registerPurchaseAdvances(services *purchaseHTTP.Services, pool *pgxpool.Pool, a auth.Resolver, p purchaseRepository.IndentsFactory, j func(database.Connection) events.Writer, cipher *documentInfra.DocumentCipher) {
	factory := func(c database.Connection) purchasePorts.AdvancesContext {
		source := p(c)
		users := authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c))
		scope := accessout.NewGate(c, accessmodule.Providers(c))
		work := purchaseRepository.NewWorkflowSourceReader(c, purchaseWorkflowProviders(c, source))
		orders := purchaseRepository.NewOrdersReleasedReader(c)
		accepted := purchaseRepository.NewAcceptanceAcceptedReader(c, purchaseRepository.AcceptanceProviders{Projects: source.Projects, Purchase: source.Scope, Access: scope, Users: users, Work: work, Orders: orders})
		gate := masterRepository.NewVendorVerificationGate(c, masterRepository.NewVendorDirectory(c), masterModule.DocumentEvidence(c, cipher, documentInfra.NewDocumentInspector(masterDomain.VendorDocumentMaximumBytes)))
		return purchaseRepository.NewAdvancesContext(purchaseRepository.AdvancesProviders{Projects: source.Projects, Purchase: source.Scope, Access: scope, Users: users, Work: work, Orders: orders, Accepted: accepted, Verification: gate, Verified: gate})
	}
	finance := func(c database.Connection) purchasePorts.AdvancesFinance {
		users := authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c))
		notices := inboxout.NewWriter(c, govout.NewDirectory(c, users), j(c), inboxout.Values{})
		return purchaseRepository.NewAdvancesFinanceBridge(financeout.NewAdvanceReviewer(c, users, accessout.NewGate(c, accessmodule.Providers(c)), j(c), notices, financeout.Values{}))
	}
	services.Advances = purchaseApp.NewAdvances(purchaseRepository.NewAdvancesUnitOfWork(pool, factory, finance, j), purchaseRepository.NewAuthBridge(a), purchaseRepository.Values{})
}
