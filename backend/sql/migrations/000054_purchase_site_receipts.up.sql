BEGIN;
CREATE TABLE purchase_site_receipts(
 id uuid PRIMARY KEY,project_id uuid NOT NULL REFERENCES projects(id),package_id uuid NOT NULL UNIQUE REFERENCES purchase_work_packages(id),
 dispatch_id uuid NOT NULL REFERENCES purchase_dispatches(id),dispatch_version integer NOT NULL CHECK(dispatch_version>0),
 version integer NOT NULL CHECK(version>0),revision integer NOT NULL CHECK(revision>0),status varchar(20) NOT NULL CHECK(status IN('pending','in_transit','delivered_at_site')),
 source_snapshot jsonb NOT NULL CHECK(jsonb_typeof(source_snapshot)='object'),data jsonb NOT NULL CHECK(jsonb_typeof(data)='object'),
 is_active boolean NOT NULL DEFAULT true,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id)
);
CREATE TABLE purchase_site_receipts_events(
 id uuid PRIMARY KEY,subject_id uuid NOT NULL REFERENCES purchase_site_receipts(id),action varchar(20) NOT NULL CHECK(action IN('record','revise')),
 reason text NOT NULL CHECK(length(trim(reason)) BETWEEN 1 AND 2000),before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,created_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id)
);
CREATE INDEX purchase_site_receipts_history ON purchase_site_receipts_events(subject_id,created_at DESC);
CREATE FUNCTION protect_purchase_site_receipts() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE x purchase_dispatches;c purchase_client_clearances;q purchase_quality_cases;r purchase_inspection_reports;i purchase_inspections;d jsonb;expected jsonb;BEGIN
 SELECT * INTO x FROM purchase_dispatches WHERE id=NEW.dispatch_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=NEW.dispatch_version AND status='dispatched' AND is_active;
 IF x.id IS NULL THEN RAISE EXCEPTION 'Site receipt needs its current dispatched material';END IF;
 SELECT * INTO c FROM purchase_client_clearances WHERE id=x.clearance_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=x.clearance_version AND status='cleared' AND is_active;
 IF c.id IS NULL THEN RAISE EXCEPTION 'Site receipt needs its current cleared client MDCC';END IF;
 SELECT * INTO q FROM purchase_quality_cases WHERE id=c.quality_id AND version=c.quality_version AND is_active;
 IF q.id IS NULL OR (SELECT count(*) FROM purchase_quality_certificates WHERE case_id=q.id)<>5 OR EXISTS(SELECT 1 FROM purchase_quality_certificates WHERE case_id=q.id AND (NOT is_active OR NOT verified OR valid_until IS NOT NULL AND valid_until<(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date)) THEN RAISE EXCEPTION 'Site receipt requires current verified quality';END IF;
 SELECT * INTO r FROM purchase_inspection_reports WHERE id=q.report_id AND version=q.report_version AND status='issued' AND is_active;
 IF r.id IS NULL THEN RAISE EXCEPTION 'Site receipt needs current issued IRN';END IF;
 SELECT * INTO i FROM purchase_inspections WHERE id=r.inspection_id AND version=r.inspection_version AND status='cleared' AND is_active;
 IF i.id IS NULL OR NOT EXISTS(SELECT 1 FROM purchase_acceptances WHERE id=i.acceptance_id AND version=i.acceptance_version AND status='accepted' AND is_active) OR NOT EXISTS(SELECT 1 FROM purchase_technical_cases WHERE id=i.technical_id AND version=i.technical_version AND state IN('approved','approved_with_comments') AND is_active) THEN RAISE EXCEPTION 'Site receipt source evidence is no longer current';END IF;
 expected=x.source_snapshot||jsonb_build_object('dispatch_id',x.id,'dispatch_version',x.version,'dispatch_revision',x.revision,'dispatch_date',x.data->'dispatch'->>'dispatch_date','lr_number',x.data->'dispatch'->>'lr_number','transporter_name',x.data->'dispatch'->>'transporter_name');
 IF NEW.source_snapshot IS DISTINCT FROM expected THEN RAISE EXCEPTION 'Site receipt source must equal its current dispatch';END IF;
 IF NEW.data->>'id' IS DISTINCT FROM NEW.id::text OR NEW.data->>'project_id' IS DISTINCT FROM NEW.project_id::text OR NEW.data->>'package_id' IS DISTINCT FROM NEW.package_id::text OR NEW.data->>'version' IS DISTINCT FROM NEW.version::text OR NEW.data->>'revision' IS DISTINCT FROM NEW.revision::text OR NEW.data->>'is_active' IS DISTINCT FROM NEW.is_active::text OR NEW.data->'source_snapshot' IS DISTINCT FROM NEW.source_snapshot OR NEW.data->'receipt'->>'status' IS DISTINCT FROM NEW.status OR (NEW.data->>'created_at')::timestamptz IS DISTINCT FROM NEW.created_at OR (NEW.data->>'updated_at')::timestamptz IS DISTINCT FROM NEW.updated_at OR nullif(NEW.data->>'created_by_id','')::uuid IS DISTINCT FROM NEW.created_by_id OR nullif(NEW.data->>'updated_by_id','')::uuid IS DISTINCT FROM NEW.updated_by_id THEN RAISE EXCEPTION 'Site receipt projection mismatch';END IF;
 IF NEW.created_by_id IS NULL OR NEW.updated_by_id IS NULL THEN RAISE EXCEPTION 'Use authenticated receiving attribution';END IF;
 d=NEW.data->'receipt';
 IF NEW.status='delivered_at_site' AND (length(trim(coalesce(d->>'receiving_officer','')))=0 OR coalesce(d->>'arrival_date','')='') THEN RAISE EXCEPTION 'Delivered receipt needs arrival date and receiving officer';END IF;
 IF coalesce(d->>'packages_condition','') NOT IN('not_checked','intact','minor_damage','quarantine') OR jsonb_typeof(d->'packages_received') IS DISTINCT FROM 'number' OR (d->>'packages_received') !~ '^(0|[1-9][0-9]*)$' OR (d->>'packages_received')::numeric>2147483647 THEN RAISE EXCEPTION 'Use actual nonnegative bounded package count and condition';END IF;
 IF length(coalesce(d->>'receiving_officer',''))>300 OR length(coalesce(d->>'transit_damage_notes',''))>4000 OR length(coalesce(d->>'gate_entry_ref',''))>200 THEN RAISE EXCEPTION 'Site receipt fields exceed limits';END IF;
 IF coalesce(d->>'arrival_date','')<>'' AND ((d->>'arrival_date')::date<(x.data->'dispatch'->>'dispatch_date')::date OR (d->>'arrival_date')::date>(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date) THEN RAISE EXCEPTION 'Arrival follows actual dispatch and cannot be future';END IF;
 IF TG_OP='INSERT' AND (NEW.version<>1 OR NEW.revision<>1) THEN RAISE EXCEPTION 'Site receipt begins at version/revision one';END IF;
 IF TG_OP='UPDATE' THEN
 IF NEW.id<>OLD.id OR NEW.project_id<>OLD.project_id OR NEW.package_id<>OLD.package_id OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'Site receipt ownership/audit must be retained';END IF;
 IF NEW.revision=OLD.revision THEN
 IF OLD.status='delivered_at_site' OR NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot THEN RAISE EXCEPTION 'Delivered or obsolete site receipt needs revision';END IF;
 ELSIF NEW.revision<>OLD.revision+1 OR NEW.status<>'pending' OR d IS DISTINCT FROM jsonb_build_object('status','pending','arrival_date','','receiving_officer','','packages_received',0,'packages_condition','not_checked','transit_damage_notes','','gate_entry_ref','') THEN RAISE EXCEPTION 'Site receipt revision resets pending and retains prior evidence';END IF;
 END IF;
 RETURN NEW;END $$;
CREATE TRIGGER purchase_site_receipts_guard BEFORE INSERT OR UPDATE ON purchase_site_receipts FOR EACH ROW EXECUTE FUNCTION protect_purchase_site_receipts();
CREATE FUNCTION protect_purchase_site_receipts_history() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Site receipt and history cannot be erased';END $$;
CREATE TRIGGER purchase_site_receipts_no_delete BEFORE DELETE ON purchase_site_receipts FOR EACH ROW EXECUTE FUNCTION protect_purchase_site_receipts_history();
CREATE TRIGGER purchase_site_receipts_no_truncate BEFORE TRUNCATE ON purchase_site_receipts FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_site_receipts_history();
CREATE TRIGGER purchase_site_receipts_event_immutable BEFORE UPDATE OR DELETE ON purchase_site_receipts_events FOR EACH ROW EXECUTE FUNCTION protect_purchase_site_receipts_history();
CREATE TRIGGER purchase_site_receipts_event_no_truncate BEFORE TRUNCATE ON purchase_site_receipts_events FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_site_receipts_history();
CREATE FUNCTION check_purchase_site_receipts_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_site_receipts_events WHERE subject_id=NEW.id AND after_snapshot=NEW.data AND created_by_id IS NOT DISTINCT FROM NEW.updated_by_id AND created_at=NEW.updated_at AND action=CASE WHEN TG_OP='UPDATE' AND NEW.revision>OLD.revision THEN 'revise' ELSE 'record' END AND (TG_OP='INSERT' AND before_snapshot='null'::jsonb OR TG_OP='UPDATE' AND before_snapshot=OLD.data)) THEN RAISE EXCEPTION 'Site receipt mutation requires exact immutable audit';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_site_receipts_write_integrity AFTER INSERT OR UPDATE ON purchase_site_receipts DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_site_receipts_write();
CREATE FUNCTION check_purchase_site_receipts_event() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE p uuid;BEGIN
 SELECT project_id INTO p FROM purchase_site_receipts WHERE id=NEW.subject_id;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_site_receipts' AND entity_type='purchase_site_receipts' AND entity_id=NEW.subject_id AND entity_version=(NEW.after_snapshot->>'version')::integer AND action=NEW.action AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted' AND occurred_at=NEW.created_at) THEN RAISE EXCEPTION 'Site receipt audit requires central project event';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_site_receipts_event_integrity AFTER INSERT ON purchase_site_receipts_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_site_receipts_event();
COMMIT;
