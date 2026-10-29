package integration

import (
	"context"
	"errors"
	migrate "github.com/golang-migrate/migrate/v4"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ranakdinesh/epcERP/internal/config"
	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/schema"
	"github.com/ranakdinesh/epcERP/internal/testsupport/postgres"
	"os"
	"testing"
)

func owned(t *testing.T) (string, *pgxpool.Pool) {
	t.Helper()
	if os.Getenv("EPCERP_GO_INTEGRATION") != "1" {
		t.Skip("run make verify for Go-owned PostgreSQL integration")
	}
	return postgres.Start(t)
}
func apply(t *testing.T, dsn string) {
	t.Helper()
	m, err := schema.Open(context.Background(), dsn)
	if err != nil {
		t.Fatal("migration initialization failed", err)
	}
	defer m.Close()
	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatal("migration application failed", err)
	}
}
func TestIndependentMigrationLifecycle(t *testing.T) {
	dsn, pool := owned(t)
	ctx := context.Background()
	// An unrelated existing schema must be rejected without creating version tables.
	if _, err := pool.Exec(ctx, "CREATE TABLE unrelated_record(id integer)"); err != nil {
		t.Fatal(err)
	}
	if _, err := schema.Open(ctx, dsn); !errors.Is(err, schema.ErrUnowned) {
		t.Fatal("unowned schema accepted")
	}
	var exists bool
	pool.QueryRow(ctx, "SELECT to_regclass('schema_migrations') IS NOT NULL").Scan(&exists)
	if exists {
		t.Fatal("rejected database mutated")
	}
	if _, err := pool.Exec(ctx, "DROP TABLE unrelated_record"); err != nil {
		t.Fatal(err)
	}
	apply(t, dsn)
	for _, table := range []string{"finance_accounts_acceptance", "finance_accounts_acceptance_events", "purchase_engineer_confirmations", "purchase_engineer_confirmations_events", "purchase_grns", "purchase_grns_events", "purchase_site_receipts", "purchase_site_receipts_events", "purchase_dispatches", "purchase_dispatch_events", "purchase_client_clearances", "purchase_client_clearance_events", "purchase_quality_cases", "purchase_quality_certificates", "purchase_quality_events", "purchase_inspection_reports", "purchase_inspection_report_events", "purchase_inspections", "purchase_inspection_events", "purchase_advances", "purchase_advance_events", "finance_advance_decisions", "purchase_acceptances", "purchase_acceptance_events", "purchase_order_document_events", "purchase_orders", "purchase_order_events", "purchase_order_sequences", "purchase_technical_cases", "purchase_technical_events", "purchase_technical_files", "purchase_technical_file_events", "project_responsibility_profiles", "project_responsibility_events", "project_budgets", "project_budget_heads", "project_budget_allocations", "project_budget_events", "organisation_director_designation", "organisation_director_events", "communication_notices", "communication_notice_events"} {
		if e := pool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&exists); e != nil || !exists {
			t.Fatal("missing governed table", table, e)
		}
	}

	if e := pool.QueryRow(ctx, "SELECT to_regclass('purchase_work_packages_assignment_lookup') IS NOT NULL").Scan(&exists); e != nil || !exists {
		t.Fatal("missing task lookup index", e)
	}
	apply(t, dsn)
	m, err := schema.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	v, dirty, err := m.Version()
	if err != nil || dirty || v != schema.LatestVersion {
		t.Fatal("unexpected migration version", v, dirty, err)
	}
	for _, table := range []string{"auth.users", "auth.roles", "auth.permissions", "auth.refresh_tokens", "client_types", "industries", "indian_states", "indian_cities", "clients", "client_contacts", "stored_documents", "stored_document_events", "material_catalogue", "manufacturers", "manufacturers_events", "business_events", "event_deliveries", "event_delivery_attempts", "projects", "project_manager_history", "auth.user_profile", "suppliers", "supplier_documents", "supplier_verifications", "supplier_verification_history", "categories", "category_subcategories", "catalogue_units", "category_catalogue_changes", "configuration_templates", "template_fields", "template_options", "template_publications", "manufacturer_imports", "catalogue_items", "catalogue_item_events", "tender.tender_fms_intakes", "document_readings", "document_reading_results", "document_reading_events", "tender.tender_fms_events", "purchase_fms_workspaces", "purchase_fms_owner_history", "work_calendar_holidays", "work_calendar_changes", "project_schedule_changes", "project_imports", "project_metadata_changes", "client_purchase_rules", "client_purchase_rule_events", "project_material_requirements", "project_material_events", "purchase_bo_lists", "purchase_bo_lines", "purchase_bo_events", "purchase_holds", "purchase_hold_events", "purchase_indents", "purchase_indent_events", "purchase_indent_releases", "purchase_indent_group_sets", "purchase_indent_group_entries", "purchase_indent_group_events", "purchase_indent_documents", "purchase_indent_document_events", "purchase_indent_dispatch_notices"} {
		if err = pool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&exists); err != nil || !exists {
			t.Fatal("missing Go table", table, err)
		}
	}
	if err = m.Steps(-1); err != nil {
		t.Fatal("bounded rollback failed", err)
	}
	if version, dirty, err := m.Version(); err != nil || dirty || version != uint(schema.LatestVersion-1) {
		t.Fatal("bounded rollback did not reach the preceding clean version", err)
	}
	if err = m.Up(); err != nil {
		t.Fatal("reapply failed", err)
	}
	if err = m.Steps(-schema.LatestVersion); err != nil {
		t.Fatal("fresh-history rollback failed", err)
	}
	var count int
	pool.QueryRow(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name <> 'schema_migrations'").Scan(&count)
	if count != 0 {
		t.Fatal("history rollback left owned tables", count)
	}
	if err = m.Up(); err != nil {
		t.Fatal("full-history reapply failed", err)
	}
}

func TestUserProfileRenamePreservesData(t *testing.T) {
	dsn, pool := owned(t)
	ctx := context.Background()
	m, err := schema.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err = m.Migrate(9); err != nil {
		t.Fatal(err)
	}
	const id = "1ae71cfa-9cf9-46b9-b0dd-077c07838b10"
	if _, err = pool.Exec(ctx, "INSERT INTO public.users(id,username,email,password_hash,is_active,is_superuser) VALUES ($1,'rename-fixture','rename@example.test','unusable',true,false)", id); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "INSERT INTO user_profiles(user_id,display_name,photo,version,created_by_id,updated_by_id) VALUES ($1,'Rename fixture',decode('010203','hex'),7,$1,$1)", id); err != nil {
		t.Fatal(err)
	}
	const eventID = "55f77277-4517-465d-b9dd-4ac4b95d1591"
	if _, err = pool.Exec(ctx, `INSERT INTO business_events(id,source_module,entity_type,entity_id,entity_version,delivery_count,action,actor_id,actor_kind,correlation_id,scope_kind,visibility,occurred_at)
		VALUES($1,'profiles','user_profile',$2,7,0,'details_updated',$2,'user',$1,'platform','restricted',now())`, eventID, id); err != nil {
		t.Fatal(err)
	}
	var originalEvent string
	if err = pool.QueryRow(ctx, "SELECT row_to_json(e)::text FROM business_events e WHERE id=$1", eventID).Scan(&originalEvent); err != nil {
		t.Fatal(err)
	}
	var original string
	// Canonical JSON compares every original column, including timestamps and nulls.
	// Migration 60 adds nullable object-reference fields without rewriting legacy data.
	if err = pool.QueryRow(ctx, "SELECT to_jsonb(p)::text FROM user_profiles p WHERE user_id=$1", id).Scan(&original); err != nil {
		t.Fatal(err)
	}
	check := func(table string) {
		t.Helper()
		var actual string
		if err := pool.QueryRow(ctx, "SELECT (to_jsonb(p)-'photo_object_key'-'photo_sha256')::text FROM "+table+" p WHERE user_id=$1", id).Scan(&actual); err != nil || actual != original {
			t.Fatal("rename altered stored profile", err)
		}
		var actualEvent string
		if err := pool.QueryRow(ctx, "SELECT row_to_json(e)::text FROM business_events e WHERE id=$1", eventID).Scan(&actualEvent); err != nil || actualEvent != originalEvent {
			t.Fatal("rename altered immutable historical event", err)
		}
		var foreignKeys int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM pg_constraint WHERE conrelid=to_regclass($1) AND contype='f'", table).Scan(&foreignKeys); err != nil || foreignKeys != 3 {
			t.Fatal("rename lost profile foreign keys", err)
		}
	}
	if err = m.Migrate(10); err != nil {
		t.Fatal(err)
	}
	check("user_profile")
	if err = m.Steps(-1); err != nil {
		t.Fatal(err)
	}
	check("user_profiles")
	if err = m.Up(); err != nil {
		t.Fatal(err)
	}
	check("auth.user_profile")
}

// Pool setup verifies connectivity; provisioning belongs to the migration command.
func TestOpenDatabaseWithoutSchema(t *testing.T) {
	dsn, _ := owned(t)
	settings, err := config.Load(func(key string) (string, bool) {
		if key == "EPCERP_DATABASE_URL" {
			return dsn, true
		}
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := infrastructure.OpenDatabase(context.Background(), settings)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var count int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'").Scan(&count); err != nil || count != 0 {
		t.Fatal("opening the database must not create schema", count, err)
	}
}
