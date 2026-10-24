package inbound

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/logging"
	"github.com/ranakdinesh/epcERP/internal/modules/document_reading/application"
	"time"
)

func Run(ctx context.Context, w *application.Worker, logger logging.Logger) error {
	for {
		if e := ctx.Err(); e != nil {
			return nil
		}
		worked, e := w.Once(ctx)
		if e != nil {
			logger.Error().Str("event", "document_reading.attempt.failed").Msg("Reading transaction failed; lease recovery remains available")
		}
		if worked && e == nil {
			logger.Info().Str("event", "document_reading.attempt.finished").Msg("Document reading attempt committed")
		}
		if !worked || e != nil {
			timer := time.NewTimer(2 * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil
			case <-timer.C:
			}
		}
	}
}
