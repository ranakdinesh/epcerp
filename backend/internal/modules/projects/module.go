// Package projects exposes the Projects capability composition boundary.
package projects

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	master "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	access "github.com/ranakdinesh/epcERP/internal/modules/project_access/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/inbound"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/outbound"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/application"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"
	purchase "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	calendar "github.com/ranakdinesh/epcERP/internal/modules/work_calendar/contracts"
)

type Dependencies struct {
	Router              chi.Router
	Database            *pgxpool.Pool
	Identity            auth.Resolver
	Clients             func(database.Connection) master.ClientDirectory
	Assignments         func(database.Connection) auth.AssignmentDirectory
	Events              func(database.Connection) events.Writer
	Calendar            func(database.Connection) calendar.Reader
	Purchasing          func(database.Connection) purchase.WorkspaceOwnership
	ReadableAssignments func(database.Connection) access.ReadableProjects
}

type Module struct{}

func New(dependencies Dependencies) *Module {
	unit := outbound.NewUnitOfWork(
		dependencies.Database,
		dependencies.Clients,
		dependencies.Assignments,
		dependencies.Events,
		dependencies.Calendar,
	).WithPurchasing(dependencies.Purchasing).WithReadableAssignments(dependencies.ReadableAssignments)
	service := application.New(unit, outbound.NewAuthBridge(dependencies.Identity), outbound.SystemValues{})
	inbound.Register(dependencies.Router, service)
	inbound.RegisterMetadata(dependencies.Router, service.Metadata())
	return &Module{}
}

func ContextReader(connection database.Connection) contracts.ContextReader {
	return outbound.NewContextReader(connection)
}

func CalendarUpdater(connection database.Connection, writer events.Writer, reader calendar.Reader) func(context.Context, string) (int, error) {
	return outbound.CalendarUpdater{Conn: connection, Events: writer, Reader: func(database.Connection) calendar.Reader { return reader }}.Recalculate
}

func WorkInventory(c database.Connection) auth.WorkSource { return outbound.NewWorkInventory(c) }
