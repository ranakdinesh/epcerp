package composition

import (
	"github.com/jackc/pgx/v5/pgxpool"

	documentInfra "github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authout "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authapp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	inboxout "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/adapters/outbound"
	inbox "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/contracts"
	eventsout "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventsapp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	financeout "github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/adapters/outbound"
	masterModule "github.com/ranakdinesh/epcERP/internal/modules/master"
	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	govout "github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/adapters/outbound"
	accessmodule "github.com/ranakdinesh/epcERP/internal/modules/project_access"
	accessout "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound"
	budgetout "github.com/ranakdinesh/epcERP/internal/modules/project_budgets/adapters/outbound"
	projectout "github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/outbound"
	purchaseHTTP "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	purchasePorts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

func quotationApprovalProviders(c database.Connection, cipher *documentInfra.DocumentCipher) purchaseRepository.QuotationApprovalsProviders {
	projects := projectout.NewContextReader(c)
	clients := masterApplication.NewClientDirectory(masterRepository.NewClientIdentityReader(c))
	purchase := purchaseRepository.NewWorkspaceAccessGate(c, projects, clients)
	quotes := purchaseRepository.QuotationsProviders{Scope: purchase, BO: purchaseRepository.NewBOReader(c), Documents: purchaseRepository.NewIndentDocumentsReader(c, purchaseRepository.NewIndentGroupsReader(c)), Projects: projects, Clients: clients, Suppliers: masterRepository.NewVendorDirectory(c)}
	return purchaseRepository.QuotationApprovalsProviders{Users: authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c)), Projects: projects, Purchase: purchase, Accounts: accessout.NewGate(c, accessmodule.Providers(c)), Quotes: purchaseRepository.NewQuotationsApprovalReader(c, quotes, purchaseRepository.NewIndentGroupsReader(c), purchaseRepository.Values{}), Budgets: budgetout.NewApprovedReader(c, projects), Verification: masterRepository.NewVendorVerificationGate(c, masterRepository.NewVendorDirectory(c), masterModule.DocumentEvidence(c, cipher, documentInfra.NewDocumentInspector(masterDomain.VendorDocumentMaximumBytes)))}
}
func registerQuotationApprovals(services *purchaseHTTP.Services, pool *pgxpool.Pool, identity auth.Resolver, cipher *documentInfra.DocumentCipher) {
	journal := func(c database.Connection) events.Writer { return eventsapp.NewRecorder(eventsout.NewJournal(c)) }
	finance := func(c database.Connection) purchasePorts.QuotationApprovalsFinance {
		users := authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c))
		notices := inboxout.NewWriter(c, govout.NewDirectory(c, users), journal(c), inboxout.Values{})
		return purchaseRepository.NewQuotationApprovalsFinanceBridge(financeout.NewQuotationReviewer(c, users, accessout.NewGate(c, accessmodule.Providers(c)), journal(c), notices, financeout.Values{}))
	}
	services.QuotationApprovals = purchaseApp.NewQuotationApprovals(purchaseRepository.NewQuotationApprovalsUnitOfWork(pool, func(c database.Connection) purchaseRepository.QuotationApprovalsProviders {
		return quotationApprovalProviders(c, cipher)
	}, journal, finance).WithNotices(func(c database.Connection) inbox.AddressedWriter {
		return inboxout.NewAddressedWriter(c, journal(c), inboxout.Values{})
	}), purchaseRepository.NewAuthBridge(identity), purchaseRepository.Values{}).WithPrinter(purchaseRepository.QuotationApprovalPrinter{})
}
