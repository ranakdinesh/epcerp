// Package purchase_fms exposes the Purchase FMS module composition boundary.
package purchase_fms

import (
	workprojects "github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	purchaseHTTP "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx"
	"github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	"github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/composition"
	"github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
)

type TaskProviders = composition.TaskProviders
type ProjectItemProviders = composition.ProjectItemProviders
type AccountsProviders = composition.AccountsProviders

type Module struct {
	UploadGuard   func(http.Handler) http.Handler
	TaskProviders TaskProviders
	ProjectItems  ProjectItemProviders
	Accounts      AccountsProviders
}

func WorkspaceOwnership(connection database.Connection, assignments auth.AssignmentDirectory, writer events.Writer) contracts.WorkspaceOwnership {
	return repository.NewWorkspaceOwnership(connection, assignments, writer)
}

func New(router *infrastructure.Router, database *pgxpool.Pool, identity auth.Resolver) (*Module, error) {
	result, err := composition.New(router, database, identity)
	if err != nil {
		return nil, err
	}
	module := &Module{UploadGuard: result.UploadGuard, TaskProviders: result.TaskProviders, ProjectItems: result.ProjectItems, Accounts: result.Accounts}
	router.AddPayloadPolicy(func(request *http.Request) bool {
		return request.Method == http.MethodPost && purchaseHTTP.IsUploadPath(request.URL.Path)
	}, 11<<20, module.UploadGuard)
	return module, nil
}

func WorkInventory(c database.Connection, parents workprojects.ContextReader) auth.WorkSource {
	return repository.NewWorkInventory(c, parents)
}
