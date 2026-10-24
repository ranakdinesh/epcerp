package organisation_governance

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authout "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authapp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	eventsout "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventsapp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	in "github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/adapters/inbound"
	out "github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/adapters/outbound"
	app "github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/application"
)

func New(r chi.Router, pool *pgxpool.Pool, identity auth.Resolver) {
	users := func(c database.Connection) auth.AssignmentDirectory {
		return authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c))
	}
	journal := func(c database.Connection) events.Writer { return eventsapp.NewRecorder(eventsout.NewJournal(c)) }
	in.Register(r, app.New(out.NewUnitOfWork(pool, users, journal), out.NewAuthBridge(identity), out.Values{}))
}

func WorkInventory(c database.Connection) auth.WorkSource { return out.NewWorkInventory(c) }
