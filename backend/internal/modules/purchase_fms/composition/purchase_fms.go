// Command api is the composition root for the independent epcERP runtime.
package composition

import (
	documentInfra "github.com/ranakdinesh/epcERP/internal/infrastructure"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authOutbound "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authApplication "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	authContracts "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	eventsOutbound "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventsApplication "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	eventsContracts "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	projectsOutbound "github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/outbound"
	projectContracts "github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"
	purchaseHTTP "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
)

type Result struct {
	UploadGuard   func(http.Handler) http.Handler
	TaskProviders TaskProviders
	ProjectItems  ProjectItemProviders
	Accounts      AccountsProviders
}

// New constructs the complete purchase unit; HTTP receives services only.
func New(mux chi.Router, pool *pgxpool.Pool, identity authContracts.Resolver) (Result, error) {
	purchaseServices := &purchaseHTTP.Services{}
	purchaseServices.ClientRules = purchaseApp.NewClientRules(purchaseRepository.NewClientRulesUnitOfWork(pool, func(c database.Connection) purchaseRepository.ClientRulesProviders {
		return purchaseRepository.ClientRulesProviders{Clients: masterApplication.NewClientDirectory(masterRepository.NewClientIdentityReader(c)), Projects: projectsOutbound.NewContextReader(c), Categories: masterRepository.NewCategoryDirectory(c), Items: masterRepository.NewCategoryDirectory(c), Makes: masterRepository.NewManufacturerItemReader(c)}
	}, func(c database.Connection) eventsContracts.Writer {
		return eventsApplication.NewRecorder(eventsOutbound.NewJournal(c))
	}), purchaseRepository.NewAuthBridge(identity), purchaseRepository.Values{})
	cipher, err := documentInfra.NewDocumentCipher(os.Getenv("EPCERP_SUPPLIER_DOCUMENT_KEY"))
	if err != nil {
		return Result{}, err
	}
	workspaceUOW := purchaseRepository.NewWorkspaceUnitOfWork(pool,
		func(c database.Connection) projectContracts.ContextReader {
			return projectsOutbound.NewContextReader(c)
		},
		func(c database.Connection) authContracts.AssignmentDirectory {
			return authApplication.NewAssignmentDirectory(authOutbound.NewAssignmentReader(c))
		},
		func(c database.Connection) eventsContracts.Writer {
			return eventsApplication.NewRecorder(eventsOutbound.NewJournal(c))
		},
	).WithApprovals(func(c database.Connection) purchaseRepository.QuotationApprovalsProviders {
		return quotationApprovalProviders(c, cipher)
	})
	guard, taskProviders, projectItems, accounts, err := registerPurchaseExecution(mux, purchaseServices, pool, identity, workspaceUOW)
	if err != nil {
		return Result{}, err
	}
	purchaseServices.Workspace = purchaseApp.NewWorkspace(workspaceUOW, purchaseRepository.NewAuthBridge(identity), purchaseRepository.Values{})
	purchaseHTTP.Register(mux, *purchaseServices)

	return Result{UploadGuard: guard, TaskProviders: taskProviders, ProjectItems: projectItems, Accounts: accounts}, nil
}
