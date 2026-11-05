BEGIN;
CREATE TABLE purchase_inspection_reports(
 id uuid PRIMARY KEY,project_id uuid NOT NULL REFERENCES projects(id),package_id uuid NOT NULL UNIQUE REFERENCES purchase_work_packages(id),
 inspection_id uuid NOT NULL REFERENCES purchase_inspections(id),inspection_version integer NOT NULL CHECK(inspection_version>0),
 version integer NOT NULL CHECK(version>0),revision integer NOT NULL CHECK(revision>0),status varchar(20) NOT NULL CHECK(status IN('pending','issued')),
 source_snapshot jsonb NOT NULL CHECK(jsonb_typeof(source_snapshot)='object'),data jsonb NOT NULL CHECK(jsonb_typeof(data)='object'),
 is_active boolean NOT NULL DEFAULT true,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id)
);
CREATE TABLE purchase_inspection_report_events(
 id uuid PRIMARY KEY,subject_id uuid NOT NULL REFERENCES purchase_inspection_reports(id),action varchar(20) NOT NULL CHECK(action IN('record','revise')),
 reason text NOT NULL CHECK(length(trim(reason)) BETWEEN 1 AND 2000),before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,created_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id)
);
CREATE INDEX purchase_inspection_report_history ON purchase_inspection_report_events(subject_id,created_at DESC);
CREATE FUNCTION protect_purchase_inspection_report() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE i purchase_inspections;a purchase_acceptances;t purchase_technical_cases;r jsonb;expected jsonb;qty numeric;BEGIN
 SELECT * INTO i FROM purchase_inspections WHERE id=NEW.inspection_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=NEW.inspection_version AND status='cleared' AND is_active;
 IF i.id IS NULL THEN RAISE EXCEPTION 'IR / IRN requires its current cleared inspection';END IF;
 SELECT * INTO a FROM purchase_acceptances WHERE id=i.acceptance_id AND version=i.acceptance_version AND status='accepted' AND is_active;
 SELECT * INTO t FROM purchase_technical_cases WHERE id=i.technical_id AND version=i.technical_version AND state IN('approved','approved_with_comments') AND is_active;
 IF a.id IS NULL OR t.id IS NULL THEN RAISE EXCEPTION 'Inspection report source evidence is no longer current';END IF;
 expected=jsonb_build_object('project_id',NEW.project_id,'package_id',NEW.package_id,'inspection_id',i.id,'inspection_version',i.version,'inspection_revision',i.revision,'actual_date',i.data->'inspection'->>'actual_date','order_id',i.source_snapshot->>'order_id','order_version',i.source_snapshot->'order_version','number',i.source_snapshot->>'number','supplier_name',i.source_snapshot->>'supplier_name','description',i.source_snapshot->>'description','quantity',i.source_snapshot->>'quantity','uom',i.source_snapshot->>'uom','qap_reference',i.source_snapshot->>'qap_reference','available',true,'blockers','[]'::jsonb);
 IF NEW.source_snapshot IS DISTINCT FROM expected THEN RAISE EXCEPTION 'Report source must equal its current inspected material';END IF;
 IF NEW.data->>'id' IS DISTINCT FROM NEW.id::text OR NEW.data->>'project_id' IS DISTINCT FROM NEW.project_id::text OR NEW.data->>'package_id' IS DISTINCT FROM NEW.package_id::text OR NEW.data->>'version' IS DISTINCT FROM NEW.version::text OR NEW.data->>'revision' IS DISTINCT FROM NEW.revision::text OR NEW.data->>'is_active' IS DISTINCT FROM NEW.is_active::text OR NEW.data->'source_snapshot' IS DISTINCT FROM NEW.source_snapshot OR NEW.data->'report'->>'status' IS DISTINCT FROM NEW.status THEN RAISE EXCEPTION 'Inspection report projection mismatch';END IF;
 r=NEW.data->'report';
 IF r->>'uom' IS DISTINCT FROM i.source_snapshot->>'uom' THEN RAISE EXCEPTION 'Report unit is inherited from inspected material';END IF;
 IF coalesce(r->>'cleared_quantity','') !~ '^(0|[1-9][0-9]{0,11})(\.[0-9]{1,6})?$' THEN RAISE EXCEPTION 'Use an exact report quantity';END IF;
 qty=(r->>'cleared_quantity')::numeric;
 IF qty>(i.source_snapshot->>'quantity')::numeric OR NEW.status='issued' AND qty<=0 THEN RAISE EXCEPTION 'Report quantity must match its actual inspected material allowance';END IF;
 IF NEW.status='issued' AND (length(trim(coalesce(r->>'ir_number','')))=0 OR length(trim(coalesce(r->>'irn_number','')))=0 OR coalesce(r->>'issue_date','')='') THEN RAISE EXCEPTION 'Issued report needs IR, IRN and actual issue date';END IF;
 IF length(coalesce(r->>'ir_number',''))>200 OR length(coalesce(r->>'irn_number',''))>200 OR length(coalesce(r->>'issuing_authority',''))>300 OR length(coalesce(r->>'file_name',''))>500 THEN RAISE EXCEPTION 'Report fields exceed their limits';END IF;
 IF coalesce(r->>'issue_date','')<>'' AND ((r->>'issue_date')::date<(i.data->'inspection'->>'actual_date')::date OR (r->>'issue_date')::date>(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date) THEN RAISE EXCEPTION 'Report issue date must follow the actual inspection and cannot be future';END IF;
 IF TG_OP='INSERT' AND (NEW.version<>1 OR NEW.revision<>1) THEN RAISE EXCEPTION 'Report begins at version and revision one';END IF;
 IF TG_OP='UPDATE' THEN
  IF NEW.id<>OLD.id OR NEW.project_id<>OLD.project_id OR NEW.package_id<>OLD.package_id OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'Report ownership and audit must be retained';END IF;
  IF NEW.revision=OLD.revision THEN
   IF OLD.status='issued' OR NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot THEN RAISE EXCEPTION 'Issued or obsolete report requires a revision';END IF;
  ELSIF NEW.revision<>OLD.revision+1 OR NEW.status<>'pending' OR r IS DISTINCT FROM jsonb_build_object('status','pending','ir_number','','irn_number','','issue_date','','issuing_authority','','cleared_quantity','0','uom',i.source_snapshot->>'uom','file_name','') THEN RAISE EXCEPTION 'Report revision resets issuance and retains old evidence';END IF;
 END IF;
 RETURN NEW;END $$;
CREATE TRIGGER purchase_inspection_report_guard BEFORE INSERT OR UPDATE ON purchase_inspection_reports FOR EACH ROW EXECUTE FUNCTION protect_purchase_inspection_report();
CREATE FUNCTION protect_purchase_inspection_report_history() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'IR / IRN and history cannot be erased';END $$;
CREATE TRIGGER purchase_inspection_report_no_delete BEFORE DELETE ON purchase_inspection_reports FOR EACH ROW EXECUTE FUNCTION protect_purchase_inspection_report_history();
CREATE TRIGGER purchase_inspection_report_no_truncate BEFORE TRUNCATE ON purchase_inspection_reports FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_inspection_report_history();
CREATE TRIGGER purchase_inspection_report_event_immutable BEFORE UPDATE OR DELETE ON purchase_inspection_report_events FOR EACH ROW EXECUTE FUNCTION protect_purchase_inspection_report_history();
CREATE TRIGGER purchase_inspection_report_event_no_truncate BEFORE TRUNCATE ON purchase_inspection_report_events FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_inspection_report_history();
CREATE FUNCTION check_purchase_inspection_report_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_inspection_report_events WHERE subject_id=NEW.id AND after_snapshot=NEW.data AND created_by_id IS NOT DISTINCT FROM NEW.updated_by_id AND created_at=NEW.updated_at AND action=CASE WHEN TG_OP='UPDATE' AND NEW.revision>OLD.revision THEN 'revise' ELSE 'record' END AND (TG_OP='INSERT' AND before_snapshot='null'::jsonb OR TG_OP='UPDATE' AND before_snapshot=OLD.data)) THEN RAISE EXCEPTION 'Report mutation requires exact immutable audit';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_inspection_report_write_integrity AFTER INSERT OR UPDATE ON purchase_inspection_reports DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_inspection_report_write();
CREATE FUNCTION check_purchase_inspection_report_event() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE p uuid;BEGIN
 SELECT project_id INTO p FROM purchase_inspection_reports WHERE id=NEW.subject_id;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_inspection_reports' AND entity_type='purchase_inspection_reports' AND entity_id=NEW.subject_id AND entity_version=(NEW.after_snapshot->>'version')::integer AND action=NEW.action AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted' AND occurred_at=NEW.created_at) THEN RAISE EXCEPTION 'Report audit requires central project event';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_inspection_report_event_integrity AFTER INSERT ON purchase_inspection_report_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_inspection_report_event();
COMMIT;
