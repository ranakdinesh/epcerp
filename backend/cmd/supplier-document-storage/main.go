// Operator-only storage relocation; requires direct access to the independent Go database and MinIO.
package main

import (
	"context"
	"fmt"
	"github.com/ranakdinesh/epcERP/internal/config"
	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery"
	master "github.com/ranakdinesh/epcERP/internal/modules/master"
	"os"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Supplier storage relocation failed; source bytes retained for uncommitted records. Check configuration, migration 67 and operator access.")
		os.Exit(1)
	}
}
func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := config.LoadLocalEnvironment(); err != nil {
		return err
	}
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		return err
	}
	pool, err := infrastructure.OpenDatabase(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()
	count, err := master.RelocateDocuments(ctx, pool, events.New().Writer)
	if err != nil {
		return err
	}
	fmt.Printf("Supplier documents moved to encrypted MinIO storage: %d\n", count)
	return nil
}
