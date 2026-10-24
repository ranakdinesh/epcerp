// Package finance_accounts_acceptance exposes finance acceptance composition.
package finance_accounts_acceptance

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authout "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authapp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	inboxout "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/adapters/outbound"
	inbox "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/adapters/inbound"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/adapters/outbound"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/application"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/ports"
	govout "github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/adapters/outbound"
)

type Dependencies struct {
	Router    chi.Router
	Database  *pgxpool.Pool
	Identity  auth.Resolver
	Providers func(database.Connection) outbound.Providers
	Events    func(database.Connection) events.Writer
}

type Module struct{}

func New(dependencies Dependencies) *Module {
	factory := func(connection database.Connection) ports.Context {
		return outbound.NewContext(dependencies.Providers(connection))
	}
	unit := outbound.NewUnitOfWork(dependencies.Database, factory, dependencies.Events, func(connection database.Connection) inbox.Writer {
		users := authapp.NewAssignmentDirectory(authout.NewAssignmentReader(connection))
		return inboxout.NewWriter(connection, govout.NewDirectory(connection, users), dependencies.Events(connection), inboxout.Values{})
	})
	inbound.Register(dependencies.Router, application.New(unit, outbound.NewAuthBridge(dependencies.Identity), outbound.Values{}))
	return &Module{}
}
