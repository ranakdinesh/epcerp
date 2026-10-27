package app

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth"
	packages "github.com/ranakdinesh/epcERP/internal/modules/document_packages"
	packagecontracts "github.com/ranakdinesh/epcERP/internal/modules/document_packages/contracts"
	reading "github.com/ranakdinesh/epcERP/internal/modules/document_reading"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery"
	tender "github.com/ranakdinesh/epcERP/internal/modules/tender_fms"
	"os"
	"time"
)

func RunTSCWorker(ctx context.Context) error {
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
	identity := &auth.Module{}
	worker, e := tender.NewTSC(tender.Dependencies{Database: pool, Users: identity.Assignments, Reader: reader.Reader, PreparedReader: reader.PreparedReader, Events: journal.Writer, DraftPackages: func(c database.Connection) packagecontracts.DraftConsumer {
		return packages.DraftConsumer(c, identity.Assignments(c))
	}})
	if e != nil {
		return e
	}
	logger.Info().Str("event", "tender.tsc.worker.started").Msg("TSC draft worker started")
	for ctx.Err() == nil {
		process := func(job context.Context) (bool, error) {
			bounded, cancel := context.WithTimeout(job, 15*time.Minute)
			defer cancel()
			return worker.Once(bounded)
		}
		var worked bool
		if os.Getenv("EPCERP_TSC_MODEL") != "" {
			worked, e = infrastructure.WithDocumentSlot(ctx, pool, process)
		} else {
			// Cloud/manual answer work does not hold the CPU reader/scanner slot
			// while waiting for a network response. Durable TSC claims still fence it.
			worked, e = process(ctx)
		}
		if e != nil && ctx.Err() == nil {
			logger.Error().Str("event", "tender.tsc.worker.failed").Msg("TSC work could not complete; inspect persisted job status")
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
	return nil
}
