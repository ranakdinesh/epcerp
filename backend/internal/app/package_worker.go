package app

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth"
	packages "github.com/ranakdinesh/epcERP/internal/modules/document_packages"
	reading "github.com/ranakdinesh/epcERP/internal/modules/document_reading"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery"
	"os"
	"time"
)

func RunPackageWorker(ctx context.Context) error {
	cfg, logger, e := loadPlatform()
	if e != nil {
		return e
	}
	pool, e := infrastructure.OpenDatabase(ctx, cfg)
	if e != nil {
		return e
	}
	defer pool.Close()
	journal := events.New()
	reader, e := reading.New(pool, journal.Writer)
	if e != nil {
		return e
	}
	auth := &auth.Module{}
	module, e := packages.New(packages.Dependencies{Database: pool, Users: auth.Assignments, Preparer: reader.Preparer()})
	if e != nil {
		return e
	}
	defer module.Store.Close()
	logger.Info().Str("event", "document_packages.worker.started").Msg("Document package worker started")
	for {
		if ctx.Err() != nil {
			return nil
		}
		worked, e := module.Worker.Once(ctx)
		if e == nil {
			e = module.Worker.Maintain(ctx, module.Objects)
		}
		if e != nil && ctx.Err() == nil {
			logger.Error().Str("event", "document_packages.worker.failed").Msg("Document package verification failed; inspect source status")
		}
		if os.Getenv("EPCERP_WORKER_ONCE") == "1" {
			return e
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
