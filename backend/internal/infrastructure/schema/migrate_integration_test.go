package schema

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/ranakdinesh/epcERP/internal/testsupport/postgres"
)

func TestOwnedEmbeddedHistoryLifecycle(t *testing.T) {
	if os.Getenv("EPCERP_GO_INTEGRATION") != "1" {
		t.Skip("requires a disposable Go-owned PostgreSQL cluster")
	}
	dsn, pool := postgres.Start(t)
	m, err := Open(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	assertLatest := func() {
		t.Helper()
		version, dirty, err := m.Version()
		if err != nil || dirty || version != LatestVersion {
			t.Fatalf("unexpected migration state: version=%d dirty=%t err=%v", version, dirty, err)
		}
	}
	if err := m.Up(); err != nil {
		t.Fatal("fresh migration failed", err)
	}
	assertLatest()
	if err := m.Up(); !errors.Is(err, migrate.ErrNoChange) {
		t.Fatal("repeated up must not change the schema", err)
	}
	if err := m.Steps(-1); err != nil {
		t.Fatal("bounded rollback failed", err)
	}
	if version, dirty, err := m.Version(); err != nil || dirty || version != LatestVersion-1 {
		t.Fatal("bounded rollback did not reach preceding clean version", version, dirty, err)
	}
	if err := m.Up(); err != nil {
		t.Fatal("reapply failed", err)
	}
	assertLatest()
	if err := m.Steps(-LatestVersion); err != nil {
		t.Fatal("full fresh-history rollback failed", err)
	}
	var count int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name <> 'schema_migrations'").Scan(&count); err != nil || count != 0 {
		t.Fatal("rollback left schema tables", count, err)
	}
	if err := m.Up(); err != nil {
		t.Fatal("full-history reapply failed", err)
	}
	assertLatest()
}
