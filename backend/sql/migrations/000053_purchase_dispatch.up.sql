BEGIN;
CREATE TABLE purchase_dispatches(
 id uuid PRIMARY KEY,project_id uuid NOT NULL REFERENCES projects(id),package_id uuid NOT NULL UNIQUE REFERENCES purchase_work_packages(id),
 clearance_id uuid NOT NULL REFERENCES purchase_client_clearances(id),clearance_version integer NOT NULL CHECK(clearance_version>0),
 version integer NOT NULL CHECK(version>0),revision integer NOT NULL CHECK(revision>0),status varchar(20) NOT NULL CHECK(status IN('pending','dispatched')),
 source_snapshot jsonb NOT NULL CHECK(jsonb_typeof(source_snapshot)='object'),data jsonb NOT NULL CHECK(jsonb_typeof(data)='object'),
 is_active boolean NOT NULL DEFAULT true,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id)
);
CREATE TABLE purchase_dispatch_events(
 id uuid PRIMARY KEY,subject_id uuid NOT NULL REFERENCES purchase_dispatches(id),action varchar(20) NOT NULL CHECK(action IN('record','revise')),
 reason text NOT NULL CHECK(length(trim(reason)) BETWEEN 1 AND 2000),before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,created_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id)
);
CREATE INDEX purchase_dispatch_history ON purchase_dispatch_events(subject_id,created_at DESC);
CREATE FUNCTION protect_purchase_dispatch() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE c purchase_client_clearances;q purchase_quality_cases;r purchase_inspection_reports;i purchase_inspections;d jsonb;expected jsonb;BEGIN
 SELECT * INTO c FROM purchase_client_clearances WHERE id=NEW.clearance_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=NEW.clearance_version AND status='cleared' AND is_active;
 IF c.id IS NULL THEN RAISE EXCEPTION 'Dispatch needs its current cleared client MDCC';END IF;
 SELECT * INTO q FROM purchase_quality_cases WHERE id=c.quality_id AND version=c.quality_version AND is_active;
 IF q.id IS NULL OR (SELECT count(*) FROM purchase_quality_certificates WHERE case_id=q.id)<>5 OR EXISTS(SELECT 1 FROM purchase_quality_certificates WHERE case_id=q.id AND (NOT is_active OR NOT verified OR valid_until IS NOT NULL AND valid_until<(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date)) THEN RAISE EXCEPTION 'Dispatch requires current verified quality';END IF;
 SELECT * INTO r FROM purchase_inspection_reports WHERE id=q.report_id AND version=q.report_version AND status='issued' AND is_active;
 IF r.id IS NULL THEN RAISE EXCEPTION 'Dispatch needs current issued IRN';END IF;
 SELECT * INTO i FROM purchase_inspections WHERE id=r.inspection_id AND version=r.inspection_version AND status='cleared' AND is_active;
 IF i.id IS NULL OR NOT EXISTS(SELECT 1 FROM purchase_acceptances WHERE id=i.acceptance_id AND version=i.acceptance_version AND status='accepted' AND is_active) OR NOT EXISTS(SELECT 1 FROM purchase_technical_cases WHERE id=i.technical_id AND version=i.technical_version AND state IN('approved','approved_with_comments') AND is_active) THEN RAISE EXCEPTION 'Dispatch source evidence is no longer current';END IF;
 expected=c.source_snapshot||jsonb_build_object('clearance_id',c.id,'clearance_version',c.version,'clearance_revision',c.revision,'mdcc_ref',c.data->'clearance'->>'mdcc_ref','clearance_date',c.data->'clearance'->>'clearance_date');
 IF NEW.source_snapshot IS DISTINCT FROM expected THEN RAISE EXCEPTION 'Dispatch source must equal its current client clearance';END IF;
 IF NEW.data->>'id' IS DISTINCT FROM NEW.id::text OR NEW.data->>'project_id' IS DISTINCT FROM NEW.project_id::text OR NEW.data->>'package_id' IS DISTINCT FROM NEW.package_id::text OR NEW.data->>'version' IS DISTINCT FROM NEW.version::text OR NEW.data->>'revision' IS DISTINCT FROM NEW.revision::text OR NEW.data->>'is_active' IS DISTINCT FROM NEW.is_active::text OR NEW.data->'source_snapshot' IS DISTINCT FROM NEW.source_snapshot OR NEW.data->'dispatch'->>'status' IS DISTINCT FROM NEW.status OR (NEW.data->>'created_at')::timestamptz IS DISTINCT FROM NEW.created_at OR (NEW.data->>'updated_at')::timestamptz IS DISTINCT FROM NEW.updated_at OR nullif(NEW.data->>'created_by_id','')::uuid IS DISTINCT FROM NEW.created_by_id OR nullif(NEW.data->>'updated_by_id','')::uuid IS DISTINCT FROM NEW.updated_by_id THEN RAISE EXCEPTION 'Dispatch projection mismatch';END IF;
 IF NEW.created_by_id IS NULL OR NEW.updated_by_id IS NULL THEN RAISE EXCEPTION 'Use authenticated dispatch attribution';END IF;
 d=NEW.data->'dispatch';
 IF NEW.status='dispatched' AND (length(trim(coalesce(d->>'transporter_name','')))=0 OR length(trim(coalesce(d->>'lr_number','')))=0 OR coalesce(d->>'dispatch_date','')='') THEN RAISE EXCEPTION 'Dispatched needs transporter, LR and actual date';END IF;
 IF length(coalesce(d->>'transporter_name',''))>300 OR length(coalesce(d->>'lr_number',''))>200 OR length(coalesce(d->>'vehicle_number',''))>100 OR length(coalesce(d->>'eway_bill_number',''))>200 OR length(coalesce(d->>'invoice_number',''))>200 OR length(coalesce(d->>'driver_contact',''))>100 OR length(coalesce(d->>'file_name',''))>500 THEN RAISE EXCEPTION 'Dispatch fields exceed limits';END IF;
 IF coalesce(d->>'dispatch_date','')<>'' AND ((d->>'dispatch_date')::date<(c.data->'clearance'->>'clearance_date')::date OR (d->>'dispatch_date')::date>(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date) THEN RAISE EXCEPTION 'Dispatch date follows client clearance and cannot be future';END IF;
 IF coalesce(d->>'lr_date','')<>'' AND (d->>'lr_date')::date>(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date OR coalesce(d->>'invoice_date','')<>'' AND (d->>'invoice_date')::date>(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date THEN RAISE EXCEPTION 'Use nonfuture actual logistics document dates';END IF;
 IF coalesce(d->>'eway_bill_valid_until','')<>'' THEN PERFORM (d->>'eway_bill_valid_until')::date;END IF;
 IF TG_OP='INSERT' AND (NEW.version<>1 OR NEW.revision<>1) THEN RAISE EXCEPTION 'Dispatch begins at version/revision one';END IF;
 IF TG_OP='UPDATE' THEN
 IF NEW.id<>OLD.id OR NEW.project_id<>OLD.project_id OR NEW.package_id<>OLD.package_id OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'Dispatch ownership/audit must be retained';END IF;
 IF NEW.revision=OLD.revision THEN
 IF OLD.status='dispatched' OR NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot THEN RAISE EXCEPTION 'Dispatched or obsolete logistics needs revision';END IF;
 ELSIF NEW.revision<>OLD.revision+1 OR NEW.status<>'pending' OR d IS DISTINCT FROM jsonb_build_object('status','pending','dispatch_date','','transporter_name','','lr_number','','lr_date','','vehicle_number','','eway_bill_number','','eway_bill_valid_until','','invoice_number','','invoice_date','','driver_contact','','file_name','') THEN RAISE EXCEPTION 'Dispatch revision resets pending and retains prior evidence';END IF;
 END IF;
 RETURN NEW;END $$;
CREATE TRIGGER purchase_dispatch_guard BEFORE INSERT OR UPDATE ON purchase_dispatches FOR EACH ROW EXECUTE FUNCTION protect_purchase_dispatch();
CREATE FUNCTION protect_purchase_dispatch_history() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'IR / IRN and history cannot be erased';END $$;
CREATE TRIGGER purchase_dispatch_no_delete BEFORE DELETE ON purchase_dispatches FOR EACH ROW EXECUTE FUNCTION protect_purchase_dispatch_history();
CREATE TRIGGER purchase_dispatch_no_truncate BEFORE TRUNCATE ON purchase_dispatches FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_dispatch_history();
CREATE TRIGGER purchase_dispatch_event_immutable BEFORE UPDATE OR DELETE ON purchase_dispatch_events FOR EACH ROW EXECUTE FUNCTION protect_purchase_dispatch_history();
CREATE TRIGGER purchase_dispatch_event_no_truncate BEFORE TRUNCATE ON purchase_dispatch_events FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_dispatch_history();
CREATE FUNCTION check_purchase_dispatch_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_dispatch_events WHERE subject_id=NEW.id AND after_snapshot=NEW.data AND created_by_id IS NOT DISTINCT FROM NEW.updated_by_id AND created_at=NEW.updated_at AND action=CASE WHEN TG_OP='UPDATE' AND NEW.revision>OLD.revision THEN 'revise' ELSE 'record' END AND (TG_OP='INSERT' AND before_snapshot='null'::jsonb OR TG_OP='UPDATE' AND before_snapshot=OLD.data)) THEN RAISE EXCEPTION 'Report mutation requires exact immutable audit';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_dispatch_write_integrity AFTER INSERT OR UPDATE ON purchase_dispatches DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_dispatch_write();
CREATE FUNCTION check_purchase_dispatch_event() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE p uuid;BEGIN
 SELECT project_id INTO p FROM purchase_dispatches WHERE id=NEW.subject_id;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_dispatch' AND entity_type='purchase_dispatch' AND entity_id=NEW.subject_id AND entity_version=(NEW.after_snapshot->>'version')::integer AND action=NEW.action AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted' AND occurred_at=NEW.created_at) THEN RAISE EXCEPTION 'Report audit requires central project event';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_dispatch_event_integrity AFTER INSERT ON purchase_dispatch_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_dispatch_event();
COMMIT;
