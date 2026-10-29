package integration

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/schema"
	"testing"
)

func TestTSCCloudMigrationBoundedRollback(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	m, e := schema.Open(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	if e = m.Steps(-1); e != nil {
		t.Fatal("empty 93->92", e)
	}
	var exists bool
	if e = pool.QueryRow(ctx, "SELECT to_regclass('tender.tsc_cloud_calls') IS NOT NULL").Scan(&exists); e != nil || exists {
		t.Fatal("cloud table retained on empty rollback", e)
	}
	if e = m.Steps(1); e != nil {
		t.Fatal("92->93", e)
	}
	if e = pool.QueryRow(ctx, "SELECT to_regclass('tender.tsc_cloud_calls') IS NOT NULL").Scan(&exists); e != nil || !exists {
		t.Fatal("cloud ledger absent", e)
	}
}
