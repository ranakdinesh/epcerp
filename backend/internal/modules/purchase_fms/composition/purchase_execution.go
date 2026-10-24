package composition

import (
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	documentInfra "github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authout "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authapp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	eventsout "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventsapp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	accountsout "github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/adapters/outbound"
	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	accessmodule "github.com/ranakdinesh/epcERP/internal/modules/project_access"
	accessout "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound"
	materialout "github.com/ranakdinesh/epcERP/internal/modules/project_items/adapters/outbound"
	projectout "github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/outbound"
	purchaseHTTP "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	purchaseContracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
)

type TaskProviders struct {
	Users      func(database.Connection) auth.AssignmentDirectory
	Quotations func(database.Connection) purchaseContracts.QuotationWorkReader
	Workflow   func(database.Connection) purchaseContracts.WorkflowTaskReader
}

type ProjectItemProviders func(database.Connection) materialout.Providers
type AccountsProviders func(database.Connection) accountsout.Providers

// Composition is the only place that joins capability adapters. Every provider
// receives its caller's connection so source gates, records and audit commit together.
func registerPurchaseExecution(router chi.Router, services *purchaseHTTP.Services, pool *pgxpool.Pool, identity auth.Resolver, workspace *purchaseRepository.WorkspaceUnitOfWork) (func(http.Handler) http.Handler, TaskProviders, ProjectItemProviders, AccountsProviders, error) {
	cipher, e := documentInfra.NewDocumentCipher(os.Getenv("EPCERP_SUPPLIER_DOCUMENT_KEY"))
	if e != nil {
		return nil, TaskProviders{}, nil, nil, e
	}
	journal := func(c database.Connection) events.Writer { return eventsapp.NewRecorder(eventsout.NewJournal(c)) }
	materials := func(c database.Connection) materialout.Providers {
		return materialout.Providers{Scope: purchaseRepository.NewWorkspaceAccessGate(c, projectout.NewContextReader(c), masterApplication.NewClientDirectory(masterRepository.NewClientIdentityReader(c))), Items: masterRepository.NewItemReader(c, masterRepository.NewCategoryDirectory(c), masterRepository.NewTemplateItemReader(c), masterRepository.NewManufacturerItemReader(c)), Rules: purchaseRepository.NewClientRulesEvaluator(c), Projects: projectout.NewContextReader(c), Commitments: purchaseRepository.NewIndentsReader(c)}
	}
	services.Holds = purchaseApp.NewHolds(purchaseRepository.NewHoldsUnitOfWork(pool, func(c database.Connection) purchaseRepository.HoldsProviders {
		p := materials(c)
		return purchaseRepository.HoldsProviders{Scope: purchaseRepository.NewWorkspaceAccessGate(c, projectout.NewContextReader(c), masterApplication.NewClientDirectory(masterRepository.NewClientIdentityReader(c))), Categories: masterRepository.NewCategoryDirectory(c), Demands: materialout.NewDemandPort(c, p, journal(c)), Users: authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c))}
	}, journal), purchaseRepository.NewAuthBridge(identity), purchaseRepository.Values{})
	services.BO = purchaseApp.NewBO(purchaseRepository.NewBOUnitOfWork(pool, func(c database.Connection) purchaseRepository.BOProviders {
		p := materials(c)
		return purchaseRepository.BOProviders{Scope: p.Scope, Items: p.Items, Rules: p.Rules, Projects: p.Projects, Demands: materialout.NewDemandPort(c, p, journal(c))}
	}, journal), purchaseRepository.NewAuthBridge(identity), purchaseRepository.Values{}, purchaseRepository.BOPrinter{})
	indentProviders := func(c database.Connection) purchaseRepository.IndentsProviders {
		p := materials(c)
		return purchaseRepository.IndentsProviders{Scope: p.Scope, Items: p.Items, Rules: p.Rules, Projects: p.Projects, Clients: masterApplication.NewClientDirectory(masterRepository.NewClientIdentityReader(c)), Demands: materialout.NewDemandPort(c, p, journal(c)), Users: authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c)), Holds: purchaseRepository.NewHoldsEvaluator(c)}
	}
	workspace.WithLaterProgress(func(c database.Connection) *purchaseRepository.LaterProgressReader {
		return purchaseRepository.NewLaterProgressReader(c, purchaseLaterProgressProviders(c, indentProviders(c), cipher))
	})
	services.Indents = purchaseApp.NewIndents(purchaseRepository.NewIndentsUnitOfWork(pool, indentProviders, journal), purchaseRepository.NewAuthBridge(identity), purchaseRepository.Values{})
	services.Awards = purchaseApp.NewAwards(purchaseRepository.NewAwardsUnitOfWork(pool, func(c database.Connection) purchaseRepository.AwardsProviders {
		p := indentProviders(c)
		return purchaseRepository.AwardsProviders{Indents: p, Workflow: purchaseWorkflowProviders(c, p), Sourcing: purchaseSourcingProviders(c, p, cipher), Approvals: quotationApprovalProviders(c, cipher)}
	}, journal), purchaseRepository.NewAuthBridge(identity), purchaseRepository.Values{})
	registerPurchaseWorkflow(services, pool, identity, indentProviders, journal)
	registerPurchaseSourcing(services, pool, identity, indentProviders, journal, cipher)
	if err := registerPurchaseOrders(services, pool, identity, indentProviders, journal, cipher); err != nil {
		return nil, TaskProviders{}, nil, nil, err
	}
	if err := registerLOIDocuments(services, pool, identity, indentProviders, journal, cipher); err != nil {
		return nil, TaskProviders{}, nil, nil, err
	}
	registerPurchaseAcceptance(services, pool, identity, indentProviders, journal)
	registerPurchaseInspections(services, pool, identity, indentProviders, journal)
	registerPurchaseInspectionReports(services, pool, identity, indentProviders, journal)
	registerPurchaseQuality(services, pool, identity, indentProviders, journal)
	registerPurchaseClientClearance(services, pool, identity, indentProviders, journal)
	registerPurchaseDispatch(services, pool, identity, indentProviders, journal)
	registerPurchaseSiteReceipts(services, pool, identity, indentProviders, journal)
	registerPurchaseGRNs(services, pool, identity, indentProviders, journal)
	registerPurchaseEngineerConfirmations(services, pool, identity, indentProviders, journal)
	registerPurchaseAdvances(services, pool, identity, indentProviders, journal, cipher)
	groups := func(c database.Connection) purchaseRepository.IndentGroupsProviders {
		p := materials(c)
		return purchaseRepository.IndentGroupsProviders{Scope: p.Scope, BO: purchaseRepository.NewBOReader(c), Suppliers: masterRepository.NewVendorDirectory(c), Projects: p.Projects, Clients: masterApplication.NewClientDirectory(masterRepository.NewClientIdentityReader(c))}
	}
	services.IndentGroups = purchaseApp.NewIndentGroups(purchaseRepository.NewIndentGroupsUnitOfWork(pool, groups, journal), purchaseRepository.NewAuthBridge(identity), purchaseRepository.Values{})
	services.IndentDocuments = purchaseApp.NewIndentDocuments(purchaseRepository.NewIndentDocumentsUnitOfWork(pool, func(c database.Connection) purchaseRepository.IndentDocumentsProviders {
		p := groups(c)
		return purchaseRepository.IndentDocumentsProviders{Scope: p.Scope, BO: p.BO, Groups: purchaseRepository.NewIndentGroupsReader(c), Commands: purchaseRepository.NewIndentGroupsCommander(c, p, journal(c)), Suppliers: p.Suppliers, Projects: p.Projects, Clients: p.Clients, Users: authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c))}
	}, journal), purchaseRepository.NewAuthBridge(identity), purchaseRepository.Values{})
	quotations := purchaseApp.NewQuotations(purchaseRepository.NewQuotationsUnitOfWork(pool, func(c database.Connection) purchaseRepository.QuotationsProviders {
		p := groups(c)
		return purchaseRepository.QuotationsProviders{Scope: p.Scope, BO: p.BO, Documents: purchaseRepository.NewIndentDocumentsReader(c, purchaseRepository.NewIndentGroupsReader(c)), Projects: p.Projects, Clients: p.Clients, Suppliers: p.Suppliers}
	}, journal), purchaseRepository.NewAuthBridge(identity), purchaseRepository.Values{}, cipher, purchaseRepository.QuotationsInspector{})
	services.Quotations = quotations
	registerQuotationApprovals(services, pool, identity, cipher)
	technicalGuard, err := registerPurchaseTechnical(services, pool, identity, indentProviders, journal, cipher)
	if err != nil {
		return nil, TaskProviders{}, nil, nil, err
	}
	quoteGuard := purchaseHTTP.QuotationsUploadGuard(quotations)
	guard := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if purchaseHTTP.OrdersIsUploadPath(r.URL.Path) {
				purchaseHTTP.OrdersUploadGuard(services.Orders)(next).ServeHTTP(w, r)
			} else if purchaseHTTP.TechnicalIsUploadPath(r.URL.Path) {
				technicalGuard(next).ServeHTTP(w, r)
			} else {
				quoteGuard(next).ServeHTTP(w, r)
			}
		})
	}
	tasks := TaskProviders{
		Users: func(c database.Connection) auth.AssignmentDirectory { return indentProviders(c).Users },
		Quotations: func(c database.Connection) purchaseContracts.QuotationWorkReader {
			providers := indentProviders(c)
			return purchaseRepository.NewQuotationWorkReader(c, providers.Users, providers.Scope, accessout.NewGate(c, accessmodule.Providers(c)), providers.Projects)
		},
		Workflow: func(c database.Connection) purchaseContracts.WorkflowTaskReader {
			providers := indentProviders(c)
			return purchaseRepository.NewWorkflowTaskReader(c, purchaseWorkflowProviders(c, providers))
		},
	}
	accounts := func(c database.Connection) accountsout.Providers {
		return purchaseAccountsProviders(c, indentProviders(c))
	}
	return guard, tasks, materials, accounts, nil

}

func purchaseAccountsProviders(connection database.Connection, providers purchaseRepository.IndentsProviders) accountsout.Providers {
	engineer := purchaseEngineerProviders(connection, providers)
	return accountsout.Providers{Projects: engineer.Projects, Purchase: engineer.Purchase, Access: engineer.Access, Users: engineer.Users, Work: engineer.Work, Engineer: purchaseRepository.NewEngineerConfirmationsConfirmedReader(connection, engineer), Orders: purchaseRepository.NewOrdersReleasedReader(connection)}
}
