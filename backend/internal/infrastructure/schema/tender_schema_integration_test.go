package schema

import (
	"context"
	"fmt"
	"github.com/ranakdinesh/epcERP/internal/testsupport/postgres"
	"os"
	"testing"
)

func TestTenderSchemaPreservesAllRecordsForeignKeysAndGuards(t *testing.T) {
	if os.Getenv("EPCERP_GO_INTEGRATION") != "1" {
		t.Skip("requires disposable Go-owned PostgreSQL")
	}
	dsn, pool := postgres.Start(t)
	ctx := context.Background()
	m, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err = m.Migrate(89); err != nil {
		t.Fatal(err)
	}
	id := func(n int) string { return fmt.Sprintf("22000000-0000-4000-8000-%012d", n) }
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := tx.Exec(ctx, q, args...); e != nil {
			t.Fatal(e)
		}
	}
	exec(`INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,'tender-schema-manager','manager@fixture.invalid','unusable',true,false),($2,'tender-schema-sales','sales@fixture.invalid','unusable',true,false)`, id(1), id(2))
	exec(`INSERT INTO public.document_package_receipts(id,owner_id)VALUES($1,$2)`, id(17), id(2))
	exec(`INSERT INTO public.document_package_files(id,receipt_id,client_file_key,filename,kind,declared_size,declared_sha256)VALUES($1,$2,$1,'fixture.pdf','pdf',1,repeat('a',64))`, id(18), id(17))
	exec(`INSERT INTO public.document_package_members(id,receipt_id,file_id,path,normalized_path,filename,kind,byte_size,expanded_bytes,sha256,reader_payload,reader_payload_sha256,validation_claim_id,scanner_revision)VALUES($1,$2,$3,'fixture.pdf','fixture.pdf','fixture.pdf','pdf',1,1,repeat('a',64),decode(repeat('00',29),'hex'),repeat('b',64),$1,'fixture')`, id(19), id(17), id(18))
	exec(`INSERT INTO public.tender_fms_intakes(id,title,owner_id,initial_sha256)VALUES($1,'Legacy fixture',$2,repeat('c',64))`, id(3), id(1))
	exec(`INSERT INTO public.tender_fms_intakes(id,title,owner_id,record_kind,sales_creator_id,manager_assignment_id,receipt_id)VALUES($1,'Named fixture',$2,'named',$3,$4,$5)`, id(4), id(1), id(2), id(5), id(17))
	exec(`INSERT INTO public.tender_fms_manager_assignments(id,intake_id,manager_id,version,reason,created_by_id)VALUES($1,$2,$3,1,'Initial fixture assignment',$4)`, id(5), id(4), id(1), id(2))
	exec(`INSERT INTO public.document_readings(id,module,entity_id,filename,format,size_bytes,sha256,original)VALUES($1,'tender_fms',$2,'fixture.pdf','pdf',1,repeat('a',64),decode('00','hex'))`, id(6), id(4))
	exec(`INSERT INTO public.tender_fms_events(id,intake_id,action,document_id,actor_id)VALUES($1,$2,'source_attached',$3,$4)`, id(7), id(3), id(6), id(2))
	exec(`INSERT INTO public.tender_fms_owner_changes(id,intake_id,previous_owner_id,owner_id,version,reason,created_by_id)VALUES($1,$2,$3,$4,1,'Fixture owner handover',$3)`, id(8), id(3), id(2), id(1))
	exec(`INSERT INTO public.tender_fms_record_events(id,intake_id,business_version,action,actor_id)VALUES($1,$2,1,'named_record_created',$3)`, id(9), id(4), id(2))
	exec(`INSERT INTO public.tender_fms_participant_grants(id,intake_id,user_id,version,is_active,reason,created_by_id)VALUES($1,$2,$3,1,true,'Fixture metadata grant',$3)`, id(10), id(4), id(2))
	exec(`INSERT INTO public.tender_provision_operations(id,tender_id,created_by_id)VALUES($1,$2,$3)`, id(11), id(4), id(2))
	exec(`INSERT INTO public.tender_capture_operations(actor_id,command,context_key,idempotency_key,fingerprint,tender_id)VALUES($1,'capture','fixture',$2,repeat('d',64),$3)`, id(2), id(12), id(4))
	exec(`INSERT INTO public.tender_source_links(member_id,tender_id,document_id,created_by_id)VALUES($1,$2,$3,$4)`, id(19), id(4), id(6), id(2))
	exec(`INSERT INTO public.business_events(id,source_module,entity_type,entity_id,entity_version,delivery_count,action,actor_id,actor_kind,correlation_id,scope_kind,visibility,occurred_at)VALUES($1,'tender_fms','tender_assignment',$2,1,0,'manager_assigned',$3,'user',$1,'platform','restricted',clock_timestamp())`, id(13), id(5), id(2))
	exec(`INSERT INTO public.tender_source_actions(id,tender_id,assignment_id,recipient_id,source_event_id,assignment_version,created_at,updated_at,created_by_id,updated_by_id) SELECT $1,$2,$3,$4,id,1,occurred_at,occurred_at,actor_id,actor_id FROM public.business_events WHERE id=$5`, id(14), id(4), id(5), id(1), id(13))
	exec(`INSERT INTO public.tender_email_jobs(id,action_id,recipient_id)VALUES($1,$2,$3)`, id(15), id(14), id(1))
	exec(`INSERT INTO public.tender_email_attempts(id,job_id,claim_id,outcome)VALUES($1,$2,$1,'claimed')`, id(16), id(15))
	exec(`CREATE TABLE public.tender_schema_fk_probe(id uuid REFERENCES public.tender_fms_intakes(id))`)
	exec(`INSERT INTO public.tender_schema_fk_probe(id)VALUES($1)`, id(4))
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	tables := []string{"tender_fms_intakes", "tender_fms_events", "tender_fms_owner_changes", "tender_fms_manager_assignments", "tender_fms_record_events", "tender_fms_participant_grants", "tender_provision_operations", "tender_capture_operations", "tender_source_links", "tender_source_actions", "tender_email_jobs", "tender_email_attempts"}
	snapshots := map[string]string{}
	oids := map[string]uint32{}
	for _, table := range tables {
		var snapshot string
		var oid uint32
		if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_agg(to_jsonb(t) ORDER BY to_jsonb(t)::text)::text FROM public.%s t`, table)).Scan(&snapshot); err != nil {
			t.Fatal(err)
		}
		if err = pool.QueryRow(ctx, `SELECT $1::regclass::oid`, "public."+table).Scan(&oid); err != nil {
			t.Fatal(err)
		}
		snapshots[table] = snapshot
		oids[table] = oid
	}
	assert := func(namespace string) {
		t.Helper()
		for _, table := range tables {
			var snapshot string
			var oid uint32
			if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_agg(to_jsonb(t) ORDER BY to_jsonb(t)::text)::text FROM %s.%s t`, namespace, table)).Scan(&snapshot); err != nil || snapshot != snapshots[table] {
				t.Fatalf("%s data changed: %v", table, err)
			}
			if err = pool.QueryRow(ctx, `SELECT $1::regclass::oid`, namespace+"."+table).Scan(&oid); err != nil || oid != oids[table] {
				t.Fatalf("%s recreated: %v", table, err)
			}
		}
	}
	if err = m.Migrate(90); err != nil {
		t.Fatal(err)
	}
	assert("tender")
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema='tender'`).Scan(&count); err != nil || count != 12 {
		t.Fatal("incomplete Tender schema", count, err)
	}
	var target string
	if err = pool.QueryRow(ctx, `SELECT confrelid::regclass::text FROM pg_constraint WHERE conrelid='public.tender_schema_fk_probe'::regclass AND contype='f'`).Scan(&target); err != nil || target != "tender.tender_fms_intakes" {
		t.Fatal("cross-schema FK lost", target, err)
	}
	for _, q := range []string{`UPDATE tender.tender_fms_manager_assignments SET reason='Tamper with source evidence'`, `DELETE FROM tender.tender_fms_record_events`, `TRUNCATE tender.tender_email_attempts`, `UPDATE tender.tender_capture_operations SET command='tamper'`} {
		if _, err = pool.Exec(ctx, q); err == nil {
			t.Fatal("immutable Tender history became mutable")
		}
	}
	if _, err = pool.Exec(ctx, `UPDATE tender.tender_fms_intakes SET owner_id=$1 WHERE id=$2`, id(2), id(4)); err == nil {
		t.Fatal("current Manager FK stopped enforcing assignment")
	}
	for _, q := range []string{`SELECT 1 FROM public.document_readings`, `SELECT 1 FROM public.document_package_receipts`, `SELECT 1 FROM public.communication_notices`, `SELECT 1 FROM public.projects`, `SELECT 1 FROM public.purchase_fms_workspaces`} {
		if _, err = pool.Exec(ctx, q); err != nil {
			t.Fatal("unrelated shared table moved", err)
		}
	}
	assert("tender")
	if err = m.Migrate(89); err != nil {
		t.Fatal(err)
	}
	assert("public")
	if err = m.Migrate(90); err != nil {
		t.Fatal(err)
	}
	assert("tender")
}
