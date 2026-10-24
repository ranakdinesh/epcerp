package schema

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/ranakdinesh/epcERP/internal/testsupport/postgres"
)

func TestAuthSchemaUpgradePreservesRecordsRelationsAndGuards(t *testing.T) {
	if os.Getenv("EPCERP_GO_INTEGRATION") != "1" {
		t.Skip("requires a disposable Go-owned PostgreSQL cluster")
	}
	dsn, pool := postgres.Start(t)
	ctx := context.Background()
	m, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := m.Migrate(84); err != nil {
		t.Fatal(err)
	}
	const manager = "11000000-0000-4000-8000-000000000001"
	const employee = "11000000-0000-4000-8000-000000000002"
	const department = "11000000-0000-4000-8000-000000000003"
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO public.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,'schema-manager','manager@fixture.invalid','unusable',true,true),($2,'schema-employee','employee@fixture.invalid','unusable',true,false)`, manager, employee)
	exec(`INSERT INTO public.auth_departments(id,code,name,manager_id) VALUES($1,'SCHEMA','Schema fixture',$2)`, department, manager)
	exec(`INSERT INTO public.auth_reporting_lines(user_id,department_id,is_company_root) VALUES($1,$2,true)`, manager, department)
	exec(`INSERT INTO public.auth_reporting_lines(user_id,department_id,manager_id) VALUES($1,$2,$3)`, employee, department, manager)
	exec(`INSERT INTO public.user_profile(user_id,display_name) VALUES($1,'Preserved profile')`, employee)
	exec(`INSERT INTO public.auth_username_changes(id,user_id,version,previous_username,username,created_by_id) VALUES('11000000-0000-4000-8000-000000000004',$1,1,'previous-employee','schema-employee',$1)`, employee)
	exec(`CREATE TABLE public.auth_schema_reference_probe(user_id uuid REFERENCES public.users(id))`)
	exec(`INSERT INTO public.auth_schema_reference_probe(user_id) VALUES($1)`, employee)
	tables := []string{"users", "roles", "permissions", "user_roles", "role_permissions", "refresh_tokens", "user_activity", "user_profile", "auth_password_recovery", "auth_credential_emails", "auth_credential_changes", "auth_departments", "auth_reporting_lines", "auth_organisation_changes", "auth_offboarding_cases", "auth_offboarding_changes", "auth_personal_contact_changes", "auth_username_changes", "auth_bootstrap_identity"}
	snapshots := map[string]string{}
	oids := map[string]uint32{}
	for _, table := range tables {
		var snapshot string
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY to_jsonb(t)::text),'[]'::jsonb)::text FROM public.%s t`, table)).Scan(&snapshot); err != nil {
			t.Fatal(err)
		}
		snapshots[table] = snapshot
		var oid uint32
		if err := pool.QueryRow(ctx, `SELECT $1::regclass::oid`, "public."+table).Scan(&oid); err != nil {
			t.Fatal(err)
		}
		oids[table] = oid
	}
	assertPreserved := func(namespace string) {
		t.Helper()
		for _, table := range tables {
			var snapshot string
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY to_jsonb(t)::text),'[]'::jsonb)::text FROM %s.%s t`, namespace, table)).Scan(&snapshot); err != nil || snapshot != snapshots[table] {
				t.Fatalf("%s records changed during schema relocation: %v", table, err)
			}
			var oid uint32
			if err := pool.QueryRow(ctx, `SELECT $1::regclass::oid`, namespace+"."+table).Scan(&oid); err != nil || oid != oids[table] {
				t.Fatalf("%s was recreated instead of moved: %v", table, err)
			}
		}
	}
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	assertPreserved("auth")
	var count int
	if err := pool.QueryRow(ctx, `SELECT CASE WHEN position('auth.users' in pg_get_functiondef('public.protect_communication_notice()'::regprocedure))>0 AND position('FROM users' in pg_get_functiondef('public.protect_communication_notice()'::regprocedure))=0 AND position('JOIN users' in pg_get_functiondef('public.protect_communication_notice()'::regprocedure))=0 THEN 1 ELSE 0 END`).Scan(&count); err != nil || count == 0 {
		t.Fatal("dependent inbox guard still resolves users in public", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema='auth'`).Scan(&count); err != nil || count != len(tables) {
		t.Fatal("incomplete Auth schema", count, err)
	}
	var target string
	if err := pool.QueryRow(ctx, `SELECT confrelid::regclass::text FROM pg_constraint WHERE conrelid='public.auth_schema_reference_probe'::regclass AND contype='f'`).Scan(&target); err != nil || target != "auth.users" {
		t.Fatal("cross-schema actor foreign key lost", target, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE auth.auth_reporting_lines SET manager_id=$1,is_company_root=false WHERE user_id=$2`, employee, manager); err == nil {
		t.Fatal("reporting-cycle trigger stopped protecting Auth")
	}
	if _, err := pool.Exec(ctx, `UPDATE auth.auth_username_changes SET username='tampered'`); err == nil {
		t.Fatal("immutable history trigger stopped protecting Auth")
	}
	assertPreserved("auth")
	if err := m.Migrate(88); err != nil {
		t.Fatal(err)
	}
	assertPreserved("public")
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	assertPreserved("auth")
}

func TestAuthNamespaceDoesNotBypassDatabaseOwnership(t *testing.T) {
	if os.Getenv("EPCERP_GO_INTEGRATION") != "1" {
		t.Skip("requires a disposable Go-owned PostgreSQL cluster")
	}
	for _, namespace := range []string{"auth", "pgx"} {
		t.Run(namespace, func(t *testing.T) {
			dsn, pool := postgres.Start(t)
			ctx := context.Background()
			if _, err := pool.Exec(ctx, fmt.Sprintf("CREATE SCHEMA %s; CREATE TABLE %s.unrelated(id integer)", namespace, namespace)); err != nil {
				t.Fatal(err)
			}
			if _, err := Open(ctx, dsn); err != ErrUnowned {
				t.Fatalf("unowned non-public data must not be accepted: %v", err)
			}
			var marker bool
			if err := pool.QueryRow(ctx, `SELECT to_regclass('public.schema_migrations') IS NOT NULL`).Scan(&marker); err != nil || marker {
				t.Fatal("ownership refusal mutated database metadata", err)
			}
		})
	}
}
