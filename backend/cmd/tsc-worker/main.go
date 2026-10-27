package main

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/app"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if app.RunTSCWorker(ctx) != nil {
		os.Exit(1)
	}
}
