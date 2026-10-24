package project_access

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	workprojects "github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authout "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authapp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	eventsout "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventsapp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	in "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/inbound"
	out "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound"
	app "github.com/ranakdinesh/epcERP/internal/modules/project_access/application"
	"github.com/ranakdinesh/epcERP/internal/modules/project_access/contracts"
	projectout "github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/outbound"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
)

func Providers(c database.Connection) out.Providers {
	users := authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c))
	return out.Providers{Projects: projectout.NewContextReader(c), Purchase: purchaseRepository.NewWorkspaceOwnership(c, users, eventsapp.NewRecorder(eventsout.NewJournal(c))), Users: users, Bundles: authout.NewBundleDirectory(c)}
}
func New(r chi.Router, pool *pgxpool.Pool, identity auth.Resolver) {
	journal := func(c database.Connection) events.Writer { return eventsapp.NewRecorder(eventsout.NewJournal(c)) }
	in.Register(r, app.New(out.NewUnitOfWork(pool, Providers, journal), out.NewAuthBridge(identity), out.Values{}))
}

func ReadableProjects(connection database.Connection) contracts.ReadableProjects {
	return out.NewReadableProjects(connection, Providers(connection))
}

func WorkInventory(c database.Connection, parents workprojects.ContextReader) auth.WorkSource {
	return out.NewWorkInventory(c, parents)
}
