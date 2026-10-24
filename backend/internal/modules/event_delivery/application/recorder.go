package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/event_delivery/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/event_delivery/ports"
)

type Recorder struct{ journal ports.Journal }

func NewRecorder(j ports.Journal) *Recorder { return &Recorder{j} }
func (r *Recorder) Record(ctx context.Context, event domain.Event, targets []domain.Target) error {
	clean, err := event.Normalize()
	if err != nil {
		return err
	}
	if err = domain.ValidateTargets(targets); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	return r.journal.Append(ctx, clean, targets)
}
