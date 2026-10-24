// Command api starts the independently composed epcERP runtime.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/ranakdinesh/epcERP/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	runtime, err := app.New(ctx)
	if err != nil {
		os.Exit(1)
	}
	defer runtime.Close()
	if err = runtime.Run(ctx); err != nil {
		runtime.Logger.Error().Str("event", "api.failed").Msg("API stopped; check configuration and port availability")
		os.Exit(1)
	}
	runtime.Logger.Info().Str("event", "api.stopped").Msg("API stopped gracefully")
}
