package database

import (
	"context"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/schema"
	"github.com/ranakdinesh/epcERP/internal/testsupport/postgres"
	"os"
	"testing"
	"time"
)

// Only run on an explicitly provisioned, Go-migrated disposable database.
// Fixtures are TEMP tables scoped to one connection, never operational records.
func TestPostgresTransactionAtomicity(t *testing.T) {

	if os.Getenv("EPCERP_GO_INTEGRATION") != "1" {
		t.Skip("run make verify for Go-owned PostgreSQL integration")
	}
	dsn, pool := postgres.Start(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	migrator, err := schema.Open(ctx, dsn)
	if err != nil {
		t.Fatal("migration setup failed")
	}
	if err = migrator.Up(); err != nil {
		migrator.Close()
		t.Fatal("migration application failed")
	}
	migrator.Close()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal("test connection unavailable")
	}
	defer conn.Release()
	var revisions int
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM epcerp_schema_identity").Scan(&revisions); err != nil || revisions != 1 {
		t.Fatal("test database must be built with epcERP Go migrations")
	}
	_, err = conn.Exec(ctx, "CREATE TEMP TABLE epcerp_transaction_fixture (kind text UNIQUE DEFERRABLE INITIALLY DEFERRED)")
	if err != nil {
		t.Fatal(err)
	}
	manager := NewManager(conn)
	denied := errors.New("fixture rejected")
	for _, mode := range []string{"success", "rejected", "canceled", "panic", "commit-failed"} {
		t.Run(mode, func(t *testing.T) {
			_, err := conn.Exec(ctx, "TRUNCATE epcerp_transaction_fixture")
			if err != nil {
				t.Fatal(err)
			}
			request, stop := context.WithCancel(ctx)
			defer stop()
			var runErr error
			panicked := false
			func() {
				defer func() {
					if value := recover(); value != nil {
						panicked = true
						if value != denied {
							t.Fatal("unexpected panic")
						}
					}
				}()
				runErr = manager.Within(request, func(workCtx context.Context, db Connection) error {
					if _, err := db.Exec(workCtx, "INSERT INTO epcerp_transaction_fixture(kind) VALUES($1)", "business"); err != nil {
						return err
					}
					audit := "audit"
					if mode == "commit-failed" {
						audit = "business"
					}
					if _, err := db.Exec(workCtx, "INSERT INTO epcerp_transaction_fixture(kind) VALUES($1)", audit); err != nil {
						return err
					}
					switch mode {
					case "rejected":
						return denied
					case "canceled":
						stop()
					case "panic":
						panic(denied)
					}
					return nil
				})
			}()
			if (mode == "panic") != panicked {
				t.Fatal("panic propagation mismatch")
			}
			if mode == "success" && runErr != nil {
				t.Fatal(runErr)
			}
			if mode == "rejected" && !errors.Is(runErr, denied) {
				t.Fatal("business error lost")
			}
			if mode == "canceled" && !errors.Is(runErr, context.Canceled) {
				t.Fatal("cancellation lost")
			}
			if mode == "commit-failed" && runErr == nil {
				t.Fatal("deferred constraint must fail at commit")
			}
			var count int
			if err := conn.QueryRow(ctx, "SELECT count(*) FROM epcerp_transaction_fixture").Scan(&count); err != nil {
				t.Fatal("connection left in failed transaction")
			}
			want := 0
			if mode == "success" {
				want = 2
			}
			if count != want {
				t.Fatalf("business/audit atomicity: got %d rows want %d", count, want)
			}
		})
	}
}
