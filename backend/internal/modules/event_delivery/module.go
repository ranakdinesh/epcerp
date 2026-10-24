// Package event_delivery exposes durable business-event infrastructure to modules.
package event_delivery

import (
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	"github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	"github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
)

type Module struct{}

func New() *Module { return &Module{} }

func (*Module) Writer(connection database.Connection) contracts.Writer {
	return application.NewRecorder(outbound.NewJournal(connection))
}
