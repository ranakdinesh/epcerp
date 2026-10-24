package main

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/config"
	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/modules/document_reading/adapters/inbound"
	"github.com/ranakdinesh/epcERP/internal/modules/document_reading/adapters/outbound"
	"github.com/ranakdinesh/epcERP/internal/modules/document_reading/application"
	eventsOutbound "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventsApplication "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	if e := config.LoadLocalEnvironment(); e != nil {
		logger, _ := infrastructure.NewLogger("info")
		logger.Error().Str("event", "document_worker.config.invalid").Msg("Invalid local worker configuration")
		os.Exit(1)
	}
	s, e := config.Load(os.LookupEnv)
	if e != nil {
		logger, _ := infrastructure.NewLogger("info")
		logger.Error().Str("event", "document_worker.config.invalid").Msg("Invalid worker configuration")
		os.Exit(1)
	}
	logger, e := infrastructure.NewLogger(s.LogLevel)
	if e != nil {
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, e := infrastructure.OpenDatabase(ctx, s)
	if e != nil {
		logger.Error().Str("event", "document_worker.database.unavailable").Msg("Worker database unavailable")
		os.Exit(1)
	}
	defer pool.Close()
	cipher, e := outbound.NewCipher(os.Getenv("EPCERP_DOCUMENT_READING_KEY"))
	if e != nil {
		logger.Error().Str("event", "document_worker.config.invalid").Msg("Invalid document encryption configuration")
		os.Exit(1)
	}
	journal := func(c database.Connection) events.Writer {
		return eventsApplication.NewRecorder(eventsOutbound.NewJournal(c))
	}
	worker := application.NewWorker(outbound.NewUnitOfWork(pool, journal), cipher, outbound.Values{}, outbound.ProcessEngine{Binary: "/app/document-reader"}, outbound.Codec{})
	logger.Info().Str("event", "document_worker.started").Msg("Document worker started")
	if e = inbound.Run(ctx, worker, logger); e != nil {
		os.Exit(1)
	}
}
