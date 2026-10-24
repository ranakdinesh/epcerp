package contracts

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/event_delivery/domain"
)

type Event = domain.Event
type Target = domain.Target

// Writer is bound to the same transaction as the source mutation by composition.
type Writer interface {
	Record(context.Context, Event, []Target) error
}
