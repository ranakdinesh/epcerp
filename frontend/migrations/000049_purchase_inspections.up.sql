BEGIN;
CREATE TABLE purchase_inspections(
 id uuid PRIMARY KEY,project_id uuid NOT NULL REFERENCES projects(id),package_id uuid NOT NULL UNIQUE REFERENCES purchase_work_packages(id),
 order_id uuid NOT NULL REFERENCES purchase_orders(id),order_version integer NOT NULL CHECK(order_version>1),
 acceptance_id uuid NOT NULL REFERENCES purchase_acceptances(id),acceptance_version integer NOT NULL CHECK(acceptance_version>0),
 technical_id uuid NOT NULL REFERENCES purchase_technical_cases(id),technical_version integer NOT NULL CHECK(technical_version>0),
 version integer NOT NULL CHECK(version>0),revision integer NOT NULL CHECK(revision>0),
 status varchar(32) NOT NULL CHECK(status IN('not_scheduled','call_received','scheduled','cleared','rejected')),
 source_snapshot jsonb NOT NULL CHECK(jsonb_typeof(source_snapshot)='object'),data jsonb NOT NULL CHECK(jsonb_typeof(data)='object'),
 is_active boolean NOT NULL DEFAULT true,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id)
);
CREATE TABLE purchase_inspection_events(
 id uuid PRIMARY KEY,subject_id uuid NOT NULL REFERENCES purchase_inspections(id),action varchar(20) NOT NULL CHECK(action IN('record','revise')),
 reason text NOT NULL CHECK(length(trim(reason)) BETWEEN 1 AND 2000),before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id)
);
CREATE INDEX purchase_inspection_history ON purchase_inspection_events(subject_id,created_at DESC);
CREATE FUNCTION protect_purchase_inspection() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE o purchase_orders;a purchase_acceptances;t purchase_technical_cases;expected jsonb;q jsonb;i jsonb;point jsonb;codes text[]='{}';day date;BEGIN
 SELECT * INTO o FROM purchase_orders WHERE id=NEW.order_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=NEW.order_version AND state='released' AND is_active;
 SELECT * INTO a FROM purchase_acceptances WHERE id=NEW.acceptance_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND order_id=NEW.order_id AND order_version=NEW.order_version AND version=NEW.acceptance_version AND status='accepted' AND is_active;
 SELECT * INTO t FROM purchase_technical_cases WHERE id=NEW.technical_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=NEW.technical_version AND state IN('approved','approved_with_comments') AND is_active;
 IF o.id IS NULL OR a.id IS NULL OR t.id IS NULL THEN RAISE EXCEPTION 'Inspection requires its current released PO, unconditional acceptance and approved technical kit';END IF;
 SELECT d INTO q FROM jsonb_array_elements(t.data->'documents') d WHERE d->>'kind'='qap' AND d->>'status'='received' AND d->>'is_active'='true';
 IF q IS NULL OR t.data->'approval'->>'qap_approved' IS DISTINCT FROM 'true' OR t.data->'approval'->>'conditions_closed' IS DISTINCT FROM 'true' OR NOT EXISTS(SELECT 1 FROM purchase_technical_files WHERE id=(q->>'file_id')::uuid AND case_id=t.id AND purpose='document' AND is_active) OR NOT EXISTS(SELECT 1 FROM purchase_technical_files WHERE id=(t.data->'approval'->>'file_id')::uuid AND case_id=t.id AND purpose='client_approval' AND is_active) THEN RAISE EXCEPTION 'Inspection requires actual approved QAP and signed client approval files';END IF;
 expected=jsonb_build_object('project_id',NEW.project_id,'package_id',NEW.package_id,'order_id',o.id,'order_version',o.version,'number',o.number,'date',o.data->'details'->>'date','supplier_id',o.source_snapshot->'supplier'->>'id','supplier_name',o.source_snapshot->'supplier'->>'name','description',o.source_snapshot->>'description','quantity',o.source_snapshot->>'quantity','uom',o.source_snapshot->>'uom','acceptance_id',a.id,'acceptance_version',a.version,'technical_id',t.id,'technical_version',t.version,'technical_revision',t.revision,'qap_reference',t.data->'approval'->>'reference','qap_date',t.data->'approval'->>'date','qap_file_id',q->>'file_id','signed_approval_file_id',t.data->'approval'->>'file_id','available',true,'blockers','[]'::jsonb);
 IF NEW.source_snapshot IS DISTINCT FROM expected THEN RAISE EXCEPTION 'Inspection source must equal its current approved evidence';END IF;
 IF NEW.data->>'id' IS DISTINCT FROM NEW.id::text OR NEW.data->>'project_id' IS DISTINCT FROM NEW.project_id::text OR NEW.data->>'package_id' IS DISTINCT FROM NEW.package_id::text OR NEW.data->>'version' IS DISTINCT FROM NEW.version::text OR NEW.data->>'revision' IS DISTINCT FROM NEW.revision::text OR NEW.data->>'is_active' IS DISTINCT FROM NEW.is_active::text OR NEW.data->'source_snapshot' IS DISTINCT FROM NEW.source_snapshot OR NEW.data->'inspection'->>'status' IS DISTINCT FROM NEW.status THEN RAISE EXCEPTION 'Inspection projection mismatch';END IF;
 i=NEW.data->'inspection';
 IF jsonb_typeof(i->'witness_points') IS DISTINCT FROM 'array' OR jsonb_array_length(i->'witness_points')>100 THEN RAISE EXCEPTION 'Inspection has a bounded witness checklist';END IF;
 IF coalesce(i->>'inspection_agency','') NOT IN('','client_consultant','tpi','internal_qc') THEN RAISE EXCEPTION 'Choose a declared inspection agency';END IF;
 IF NEW.status='call_received' AND (length(trim(coalesce(i->>'call_notice_ref','')))=0 OR coalesce(i->>'call_date','')='') THEN RAISE EXCEPTION 'Vendor inspection call needs reference and actual date';END IF;
 IF NEW.status IN('scheduled','cleared','rejected') AND length(trim(coalesce(i->>'factory_location','')))=0 THEN RAISE EXCEPTION 'Inspection requires the actual factory';END IF;
 IF NEW.status='scheduled' AND coalesce(i->>'scheduled_date','')='' AND coalesce(i->>'actual_date','')='' THEN RAISE EXCEPTION 'Schedule an inspection date';END IF;
 IF NEW.status IN('cleared','rejected') AND (coalesce(i->>'actual_date','')='' OR length(trim(coalesce(i->>'inspector_name','')))=0 OR length(trim(coalesce(i->>'findings','')))=0 OR coalesce(i->>'inspection_agency','')='') THEN RAISE EXCEPTION 'Final inspection requires actual visit, inspector, agency and findings';END IF;
 FOR point IN SELECT value FROM jsonb_array_elements(i->'witness_points') LOOP
  IF jsonb_typeof(point) IS DISTINCT FROM 'object' OR length(trim(coalesce(point->>'code','')))=0 OR length(point->>'code')>60 OR length(trim(coalesce(point->>'title','')))=0 OR length(point->>'title')>1000 OR length(coalesce(point->>'remarks',''))>2000 OR jsonb_typeof(point->'is_witness') IS DISTINCT FROM 'boolean' OR jsonb_typeof(point->'is_cleared') IS DISTINCT FROM 'boolean' THEN RAISE EXCEPTION 'Witness points require explicit valid code, title and flags';END IF;
  IF lower(point->>'code')=ANY(codes) THEN RAISE EXCEPTION 'Witness point codes must be unique';END IF;
  codes=array_append(codes,lower(point->>'code'));
  IF NEW.status='cleared' AND point->>'is_witness'='true' AND point->>'is_cleared'<>'true' THEN RAISE EXCEPTION 'Clear every mandatory QAP witness point';END IF;
 END LOOP;
 IF NEW.status='cleared' AND jsonb_array_length(i->'witness_points')=0 AND length(trim(coalesce(i->>'no_witness_points_reason','')))=0 THEN RAISE EXCEPTION 'An empty approved QAP checklist requires a clearance reason';END IF;
 IF jsonb_array_length(i->'witness_points')>0 AND coalesce(i->>'no_witness_points_reason','')<>'' THEN RAISE EXCEPTION 'No-points reason applies only to empty checklists';END IF;
 FOR q IN SELECT to_jsonb(x) FROM unnest(ARRAY['call_date','scheduled_date','actual_date']) x LOOP
  IF coalesce(i->>(q#>>'{}'),'')<>'' THEN
   day=(i->>(q#>>'{}'))::date;
   IF day<(o.data->'details'->>'date')::date OR (q#>>'{}')<>'scheduled_date' AND day>(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date OR (q#>>'{}')<>'call_date' AND coalesce(i->>'call_date','')<>'' AND day<(i->>'call_date')::date OR (q#>>'{}')='actual_date' AND day<(t.data->'approval'->>'date')::date THEN RAISE EXCEPTION 'Inspection dates must reflect actual approved evidence';END IF;
  END IF;
 END LOOP;
 IF TG_OP='INSERT' AND (NEW.version<>1 OR NEW.revision<>1) THEN RAISE EXCEPTION 'Inspection begins at version and revision one';END IF;
 IF TG_OP='UPDATE' THEN
  IF NEW.id<>OLD.id OR NEW.project_id<>OLD.project_id OR NEW.package_id<>OLD.package_id OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'Inspection ownership and audit must be retained';END IF;
  IF NEW.revision=OLD.revision THEN
   IF OLD.status IN('cleared','rejected') OR NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot THEN RAISE EXCEPTION 'Final or stale inspection requires an explicit revision';END IF;
  ELSIF NEW.revision<>OLD.revision+1 OR NEW.status<>'not_scheduled' OR i IS DISTINCT FROM jsonb_build_object('status','not_scheduled','call_notice_ref','','call_date','','scheduled_date','','actual_date','','inspection_agency','','inspector_name','','factory_location','','witness_points','[]'::jsonb,'no_witness_points_reason','','findings','') THEN RAISE EXCEPTION 'Revision must reset current results and retain old evidence';END IF;
 END IF;
 RETURN NEW;END $$;
CREATE TRIGGER purchase_inspection_guard BEFORE INSERT OR UPDATE ON purchase_inspections FOR EACH ROW EXECUTE FUNCTION protect_purchase_inspection();
CREATE FUNCTION protect_purchase_inspection_history() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Material inspection and history cannot be erased';END $$;
CREATE TRIGGER purchase_inspection_no_delete BEFORE DELETE ON purchase_inspections FOR EACH ROW EXECUTE FUNCTION protect_purchase_inspection_history();
CREATE TRIGGER purchase_inspection_no_truncate BEFORE TRUNCATE ON purchase_inspections FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_inspection_history();
CREATE TRIGGER purchase_inspection_event_immutable BEFORE UPDATE OR DELETE ON purchase_inspection_events FOR EACH ROW EXECUTE FUNCTION protect_purchase_inspection_history();
CREATE TRIGGER purchase_inspection_event_no_truncate BEFORE TRUNCATE ON purchase_inspection_events FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_inspection_history();
CREATE FUNCTION check_purchase_inspection_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_inspection_events WHERE subject_id=NEW.id AND after_snapshot=NEW.data AND created_by_id IS NOT DISTINCT FROM NEW.updated_by_id AND created_at=NEW.updated_at AND action=CASE WHEN TG_OP='UPDATE' AND NEW.revision>OLD.revision THEN 'revise' ELSE 'record' END AND (TG_OP='INSERT' AND before_snapshot='null'::jsonb OR TG_OP='UPDATE' AND before_snapshot=OLD.data)) THEN RAISE EXCEPTION 'Inspection mutation requires exact immutable audit';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_inspection_write_integrity AFTER INSERT OR UPDATE ON purchase_inspections DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_inspection_write();
CREATE FUNCTION check_purchase_inspection_event() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE p uuid;BEGIN
 SELECT project_id INTO p FROM purchase_inspections WHERE id=NEW.subject_id;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_inspections' AND entity_type='purchase_inspections' AND entity_id=NEW.subject_id AND entity_version=(NEW.after_snapshot->>'version')::integer AND action=NEW.action AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted' AND occurred_at=NEW.created_at) THEN RAISE EXCEPTION 'Inspection audit requires central project event';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_inspection_event_integrity AFTER INSERT ON purchase_inspection_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_inspection_event();
COMMIT;
