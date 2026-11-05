BEGIN;
CREATE TABLE purchase_quality_cases(
 id uuid PRIMARY KEY,project_id uuid NOT NULL REFERENCES projects(id),package_id uuid NOT NULL UNIQUE REFERENCES purchase_work_packages(id),
 report_id uuid NOT NULL REFERENCES purchase_inspection_reports(id),report_version integer NOT NULL CHECK(report_version>0),
 version integer NOT NULL CHECK(version>0),revision integer NOT NULL CHECK(revision>0),
 source_snapshot jsonb NOT NULL CHECK(jsonb_typeof(source_snapshot)='object'),data jsonb NOT NULL CHECK(jsonb_typeof(data)='object'),
 is_active boolean NOT NULL DEFAULT true,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id)
);
CREATE TABLE purchase_quality_certificates(
 case_id uuid NOT NULL REFERENCES purchase_quality_cases(id),code varchar(20) NOT NULL CHECK(code IN('TC','TR','MTC','RTC','Warranty')),
 title text NOT NULL,cert_number varchar(255) NOT NULL,issued_by varchar(255) NOT NULL,valid_until date,file_name varchar(500) NOT NULL,
 verified boolean NOT NULL,verified_at timestamptz,verified_by_id uuid REFERENCES users(id),data jsonb NOT NULL CHECK(jsonb_typeof(data)='object'),
 is_active boolean NOT NULL DEFAULT true,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id),PRIMARY KEY(case_id,code),
 CHECK(verified AND verified_at IS NOT NULL AND verified_by_id IS NOT NULL AND length(trim(cert_number))>0 AND length(trim(issued_by))>0 OR NOT verified AND verified_at IS NULL AND verified_by_id IS NULL)
);
CREATE TABLE purchase_quality_events(
 id uuid PRIMARY KEY,subject_id uuid NOT NULL REFERENCES purchase_quality_cases(id),action varchar(20) NOT NULL CHECK(action IN('record','verify','revoke','revise')),certificate_code varchar(20) NOT NULL,
 reason text NOT NULL CHECK(length(trim(reason)) BETWEEN 1 AND 2000),before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,created_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id),
 CHECK(action='revise' AND certificate_code='' OR action<>'revise' AND certificate_code IN('TC','TR','MTC','RTC','Warranty'))
);
CREATE INDEX purchase_quality_history ON purchase_quality_events(subject_id,created_at DESC);
CREATE FUNCTION protect_purchase_quality_case() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE r purchase_inspection_reports;i purchase_inspections;expected jsonb;BEGIN
 SELECT * INTO r FROM purchase_inspection_reports WHERE id=NEW.report_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=NEW.report_version AND status='issued' AND is_active;
 IF r.id IS NULL THEN RAISE EXCEPTION 'Quality requires its current issued inspection report';END IF;
 SELECT * INTO i FROM purchase_inspections WHERE id=r.inspection_id AND version=r.inspection_version AND status='cleared' AND is_active;
 IF i.id IS NULL OR NOT EXISTS(SELECT 1 FROM purchase_acceptances WHERE id=i.acceptance_id AND version=i.acceptance_version AND status='accepted' AND is_active) OR NOT EXISTS(SELECT 1 FROM purchase_technical_cases WHERE id=i.technical_id AND version=i.technical_version AND state IN('approved','approved_with_comments') AND is_active) THEN RAISE EXCEPTION 'Quality inspection source evidence is no longer current';END IF;
 expected=jsonb_build_object('project_id',NEW.project_id,'package_id',NEW.package_id,'report_id',r.id,'report_version',r.version,'report_revision',r.revision,'ir_number',r.data->'report'->>'ir_number','irn_number',r.data->'report'->>'irn_number','issue_date',r.data->'report'->>'issue_date','cleared_quantity',r.data->'report'->>'cleared_quantity','uom',r.data->'report'->>'uom','order_id',r.source_snapshot->>'order_id','order_version',r.source_snapshot->'order_version','number',r.source_snapshot->>'number','supplier_name',r.source_snapshot->>'supplier_name','description',r.source_snapshot->>'description','inspection_id',r.inspection_id,'inspection_version',r.inspection_version,'available',true,'blockers','[]'::jsonb);
 IF NEW.source_snapshot IS DISTINCT FROM expected THEN RAISE EXCEPTION 'Quality source must equal its current issued IRN';END IF;
 IF NEW.data->>'id' IS DISTINCT FROM NEW.id::text OR NEW.data->>'project_id' IS DISTINCT FROM NEW.project_id::text OR NEW.data->>'package_id' IS DISTINCT FROM NEW.package_id::text OR NEW.data->>'version' IS DISTINCT FROM NEW.version::text OR NEW.data->>'revision' IS DISTINCT FROM NEW.revision::text OR NEW.data->>'is_active' IS DISTINCT FROM NEW.is_active::text OR NEW.data->'source_snapshot' IS DISTINCT FROM NEW.source_snapshot OR (NEW.data->>'created_at')::timestamptz IS DISTINCT FROM NEW.created_at OR (NEW.data->>'updated_at')::timestamptz IS DISTINCT FROM NEW.updated_at OR nullif(NEW.data->>'created_by_id','')::uuid IS DISTINCT FROM NEW.created_by_id OR nullif(NEW.data->>'updated_by_id','')::uuid IS DISTINCT FROM NEW.updated_by_id THEN RAISE EXCEPTION 'Quality case projection mismatch';END IF;
 IF jsonb_typeof(NEW.data->'certificates') IS DISTINCT FROM 'array' OR jsonb_array_length(NEW.data->'certificates')<>5 OR (SELECT count(DISTINCT x->>'code') FROM jsonb_array_elements(NEW.data->'certificates') x WHERE x->>'code' IN('TC','TR','MTC','RTC','Warranty'))<>5 THEN RAISE EXCEPTION 'Retain all five required certificates';END IF;
 IF NEW.updated_by_id IS NULL OR NEW.created_by_id IS NULL THEN RAISE EXCEPTION 'Quality requires authenticated attribution';END IF;
 IF TG_OP='INSERT' AND (NEW.version<>1 OR NEW.revision<>1) THEN RAISE EXCEPTION 'Quality begins at version and revision one';END IF;
 IF TG_OP='UPDATE' THEN
  IF NEW.id<>OLD.id OR NEW.project_id<>OLD.project_id OR NEW.package_id<>OLD.package_id OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'Quality ownership and audit must be retained';END IF;
  IF NEW.revision=OLD.revision THEN
   IF NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot THEN RAISE EXCEPTION 'Obsolete quality source requires a revision';END IF;
  ELSIF NEW.revision<>OLD.revision+1 OR NEW.source_snapshot=OLD.source_snapshot OR EXISTS(SELECT 1 FROM jsonb_array_elements(NEW.data->'certificates') x WHERE (x->>'verified')::boolean OR x->'verified_at'<>'null'::jsonb OR x->>'verified_by_id'<>'') THEN RAISE EXCEPTION 'Quality revision resets all attestations and retains history';END IF;
 END IF;
 RETURN NEW;END $$;
CREATE TRIGGER purchase_quality_case_guard BEFORE INSERT OR UPDATE ON purchase_quality_cases FOR EACH ROW EXECUTE FUNCTION protect_purchase_quality_case();
CREATE FUNCTION protect_purchase_quality_certificate() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.title IS DISTINCT FROM (CASE NEW.code WHEN 'TC' THEN 'Type Test Certificate' WHEN 'TR' THEN 'Routine Test Report' WHEN 'MTC' THEN 'Material Test Certificate' WHEN 'RTC' THEN 'Routine / Galvanizing Certificate' WHEN 'Warranty' THEN 'Warranty / Guarantee Undertaking' END) THEN RAISE EXCEPTION 'Required certificate identity is fixed';END IF;
 IF NEW.data->>'code' IS DISTINCT FROM NEW.code OR NEW.data->>'title' IS DISTINCT FROM NEW.title OR NEW.data->>'cert_number' IS DISTINCT FROM NEW.cert_number OR NEW.data->>'issued_by' IS DISTINCT FROM NEW.issued_by OR nullif(NEW.data->>'valid_until','')::date IS DISTINCT FROM NEW.valid_until OR NEW.data->>'file_name' IS DISTINCT FROM NEW.file_name OR (NEW.data->>'verified')::boolean IS DISTINCT FROM NEW.verified OR (NEW.data->>'verified_at')::timestamptz IS DISTINCT FROM NEW.verified_at OR nullif(NEW.data->>'verified_by_id','')::uuid IS DISTINCT FROM NEW.verified_by_id OR (NEW.data->>'is_active')::boolean IS DISTINCT FROM NEW.is_active OR (NEW.data->>'created_at')::timestamptz IS DISTINCT FROM NEW.created_at OR (NEW.data->>'updated_at')::timestamptz IS DISTINCT FROM NEW.updated_at OR nullif(NEW.data->>'created_by_id','')::uuid IS DISTINCT FROM NEW.created_by_id OR nullif(NEW.data->>'updated_by_id','')::uuid IS DISTINCT FROM NEW.updated_by_id THEN RAISE EXCEPTION 'Certificate projection mismatch';END IF;
 IF NEW.created_by_id IS NULL OR NEW.updated_by_id IS NULL OR NEW.verified AND (NEW.verified_at<NEW.created_at OR NEW.verified_at>NEW.updated_at OR NEW.valid_until IS NOT NULL AND NEW.valid_until<(NEW.verified_at AT TIME ZONE 'Asia/Kolkata')::date) THEN RAISE EXCEPTION 'Use actual dated quality attribution';END IF;
 IF TG_OP='UPDATE' THEN
  IF NEW.case_id<>OLD.case_id OR NEW.code<>OLD.code OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id THEN RAISE EXCEPTION 'Certificate ownership must be retained';END IF;
  IF (NEW.cert_number,NEW.issued_by,NEW.valid_until,NEW.file_name) IS DISTINCT FROM (OLD.cert_number,OLD.issued_by,OLD.valid_until,OLD.file_name) AND NEW.verified THEN RAISE EXCEPTION 'Changed certificate details require renewed verification';END IF;
 END IF;
 RETURN NEW;END $$;
CREATE TRIGGER purchase_quality_certificate_guard BEFORE INSERT OR UPDATE ON purchase_quality_certificates FOR EACH ROW EXECUTE FUNCTION protect_purchase_quality_certificate();
CREATE FUNCTION protect_purchase_quality_history() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Quality evidence and history cannot be erased';END $$;
CREATE TRIGGER purchase_quality_case_no_delete BEFORE DELETE ON purchase_quality_cases FOR EACH ROW EXECUTE FUNCTION protect_purchase_quality_history();
CREATE TRIGGER purchase_quality_case_no_truncate BEFORE TRUNCATE ON purchase_quality_cases FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_quality_history();
CREATE TRIGGER purchase_quality_certificate_no_delete BEFORE DELETE ON purchase_quality_certificates FOR EACH ROW EXECUTE FUNCTION protect_purchase_quality_history();
CREATE TRIGGER purchase_quality_certificate_no_truncate BEFORE TRUNCATE ON purchase_quality_certificates FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_quality_history();
CREATE TRIGGER purchase_quality_event_immutable BEFORE UPDATE OR DELETE ON purchase_quality_events FOR EACH ROW EXECUTE FUNCTION protect_purchase_quality_history();
CREATE TRIGGER purchase_quality_event_no_truncate BEFORE TRUNCATE ON purchase_quality_events FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_quality_history();
CREATE FUNCTION check_purchase_quality_children() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE k uuid;c purchase_quality_cases;BEGIN
 IF TG_TABLE_NAME='purchase_quality_cases' THEN k=NEW.id;ELSE k=NEW.case_id;END IF;
 SELECT * INTO c FROM purchase_quality_cases WHERE id=k;
 IF (SELECT count(*) FROM purchase_quality_certificates WHERE case_id=k)<>5 OR EXISTS(SELECT 1 FROM purchase_quality_certificates child WHERE child.case_id=k AND NOT EXISTS(SELECT 1 FROM jsonb_array_elements(c.data->'certificates') x WHERE x=child.data)) THEN RAISE EXCEPTION 'Quality child records must equal the audited case snapshot';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_quality_children_integrity AFTER INSERT OR UPDATE ON purchase_quality_cases DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_quality_children();
CREATE CONSTRAINT TRIGGER purchase_quality_child_integrity AFTER INSERT OR UPDATE ON purchase_quality_certificates DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_quality_children();
CREATE FUNCTION check_purchase_quality_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_quality_events WHERE subject_id=NEW.id AND after_snapshot=NEW.data AND created_by_id=NEW.updated_by_id AND created_at=NEW.updated_at AND (TG_OP='INSERT' AND action='record' AND before_snapshot='null'::jsonb OR TG_OP='UPDATE' AND before_snapshot=OLD.data AND (NEW.revision=OLD.revision AND action IN('record','verify','revoke') OR NEW.revision=OLD.revision+1 AND action='revise'))) THEN RAISE EXCEPTION 'Quality mutation requires exact immutable audit';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_quality_write_integrity AFTER INSERT OR UPDATE ON purchase_quality_cases DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_quality_write();
CREATE FUNCTION check_purchase_quality_event() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE p uuid;BEGIN
 SELECT project_id INTO p FROM purchase_quality_cases WHERE id=NEW.subject_id;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_quality' AND entity_type='purchase_quality' AND entity_id=NEW.subject_id AND entity_version=(NEW.after_snapshot->>'version')::integer AND action=NEW.action AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted' AND occurred_at=NEW.created_at) THEN RAISE EXCEPTION 'Quality audit requires central project event';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_quality_event_integrity AFTER INSERT ON purchase_quality_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_quality_event();
COMMIT;
