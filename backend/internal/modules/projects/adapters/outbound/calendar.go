package outbound

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/application"
	calendar "github.com/ranakdinesh/epcERP/internal/modules/work_calendar/contracts"
)

type CalendarUpdater struct {
	Conn   database.Connection
	Events events.Writer
	Reader func(database.Connection) calendar.Reader
}

func (u CalendarUpdater) Recalculate(ctx context.Context, actor string) (int, error) {
	return application.Recalculate(ctx, &repository{db.New(u.Conn), u.Events}, calendarBridge{u.Reader, u.Conn}, actor)
}
