// Package work_calendar exposes the working-calendar composition boundary.
package work_calendar

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/work_calendar/adapters/inbound"
	"github.com/ranakdinesh/epcERP/internal/modules/work_calendar/adapters/outbound"
	"github.com/ranakdinesh/epcERP/internal/modules/work_calendar/application"
	"github.com/ranakdinesh/epcERP/internal/modules/work_calendar/contracts"
)

type Dependencies struct {
	Router   chi.Router
	Database *pgxpool.Pool
	Identity auth.Resolver
	Events   func(database.Connection) events.Writer
	Refresh  func(database.Connection, contracts.Reader) func(context.Context, string) (int, error)
}

type ReaderContract = contracts.Reader

type Module struct{}

func New(dependencies Dependencies) *Module {
	reader := func(connection database.Connection) contracts.Reader {
		return application.Reader{Repo: outbound.Repository{Conn: connection}}
	}
	inbound.Register(dependencies.Router, application.Service{
		Auth: outbound.Auth{Resolver: dependencies.Identity},
		Unit: outbound.Unit{
			Manager: database.NewManager(dependencies.Database),
			Events:  dependencies.Events,
			Refresh: func(connection database.Connection) func(context.Context, string) (int, error) {
				return dependencies.Refresh(connection, reader(connection))
			},
		},
	})
	return &Module{}
}

func Reader(connection database.Connection) contracts.Reader {
	return application.Reader{Repo: outbound.Repository{Conn: connection}}
}
