// Command migrate operates only on the independently configured epcERP database.
package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/golang-migrate/migrate/v4"
	"github.com/ranakdinesh/epcERP/internal/config"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/schema"
	"os"
	"strconv"
	"time"
)

func main() {
	if err := config.LoadLocalEnvironment(); err != nil {
		fmt.Fprintln(os.Stderr, "Migration configuration failed; check the local epcERP environment")
		os.Exit(1)
	}
	if err := run(os.Args[1:], os.Getenv("EPCERP_DATABASE_URL")); err != nil {
		// No raw driver errors, DSNs or SQL data in operator output.
		fmt.Fprintln(os.Stderr, "Migration failed; check the epcERP database, ownership, migration version and operator access")
		os.Exit(1)
	}
}
func run(args []string, dsn string) error {
	if len(args) == 0 || dsn == "" {
		return errors.New("usage: migrate up | version | down <positive steps>; set EPCERP_DATABASE_URL")
	}
	steps := 0
	switch args[0] {
	case "up", "version":
		if len(args) != 1 {
			return errors.New("unexpected argument")
		}
	case "down":
		if len(args) != 2 {
			return errors.New("down requires explicit bounded steps")
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 1 || n > schema.LatestVersion {
			return errors.New("invalid steps")
		}
		steps = n
	default:
		return errors.New("unknown command")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	m, err := schema.Open(ctx, dsn)
	if err != nil {
		return err
	}
	defer m.Close()
	switch args[0] {
	case "up":
		err = m.Up()
	case "down":
		current, dirty, e := m.Version()
		if e != nil || dirty || uint(steps) > current {
			return errors.New("rollback exceeds clean applied history")
		}
		err = m.Steps(-steps)
	case "version":
		v, dirty, e := m.Version()
		if errors.Is(e, migrate.ErrNilVersion) {
			fmt.Println("version=0 dirty=false")
			return nil
		}
		if e != nil {
			return e
		}
		fmt.Printf("version=%d dirty=%t\n", v, dirty)
		return nil
	}
	if errors.Is(err, migrate.ErrNoChange) {
		err = nil
	}
	if err == nil {
		fmt.Println("Migration completed")
	}
	return err
}
