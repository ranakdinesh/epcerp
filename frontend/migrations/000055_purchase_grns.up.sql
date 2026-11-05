BEGIN;
CREATE TABLE purchase_grns(
 id uuid PRIMARY KEY,project_id uuid NOT NULL REFERENCES projects(id),package_id uuid NOT NULL UNIQUE REFERENCES purchase_work_packages(id),
 receipt_id uuid NOT NULL REFERENCES purchase_site_receipts(id),receipt_version integer NOT NULL CHECK(receipt_version>0),
 version integer NOT NULL CHECK(version>0),revision integer NOT NULL CHECK(revision>0),status varchar(20) NOT NULL CHECK(status IN('pending','issued')),
 source_snapshot jsonb NOT NULL CHECK(jsonb_typeof(source_snapshot)='object'),data jsonb NOT NULL CHECK(jsonb_typeof(data)='object'),
 is_active boolean NOT NULL DEFAULT true,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id)
);
CREATE TABLE purchase_grns_events(
 id uuid PRIMARY KEY,subject_id uuid NOT NULL REFERENCES purchase_grns(id),action varchar(20) NOT NULL CHECK(action IN('record','revise')),
 reason text NOT NULL CHECK(length(trim(reason)) BETWEEN 1 AND 2000),before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,created_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id)
);
CREATE INDEX purchase_grns_history ON purchase_grns_events(subject_id,created_at DESC);
CREATE FUNCTION protect_purchase_grns() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE y purchase_site_receipts;x purchase_dispatches;c purchase_client_clearances;q purchase_quality_cases;r purchase_inspection_reports;i purchase_inspections;d jsonb;expected jsonb;BEGIN
 SELECT * INTO y FROM purchase_site_receipts WHERE id=NEW.receipt_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=NEW.receipt_version AND status='delivered_at_site' AND is_active;
 IF y.id IS NULL THEN RAISE EXCEPTION 'GRN needs its current delivered GRN';END IF;
 SELECT * INTO x FROM purchase_dispatches WHERE id=y.dispatch_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=y.dispatch_version AND status='dispatched' AND is_active;
 IF x.id IS NULL THEN RAISE EXCEPTION 'GRN needs its current dispatched material';END IF;
 SELECT * INTO c FROM purchase_client_clearances WHERE id=x.clearance_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=x.clearance_version AND status='cleared' AND is_active;
 IF c.id IS NULL THEN RAISE EXCEPTION 'GRN needs its current cleared client MDCC';END IF;
 SELECT * INTO q FROM purchase_quality_cases WHERE id=c.quality_id AND version=c.quality_version AND is_active;
 IF q.id IS NULL OR (SELECT count(*) FROM purchase_quality_certificates WHERE case_id=q.id)<>5 OR EXISTS(SELECT 1 FROM purchase_quality_certificates WHERE case_id=q.id AND (NOT is_active OR NOT verified OR valid_until IS NOT NULL AND valid_until<(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date)) THEN RAISE EXCEPTION 'GRN requires current verified quality';END IF;
 SELECT * INTO r FROM purchase_inspection_reports WHERE id=q.report_id AND version=q.report_version AND status='issued' AND is_active;
 IF r.id IS NULL THEN RAISE EXCEPTION 'GRN needs current issued IRN';END IF;
 SELECT * INTO i FROM purchase_inspections WHERE id=r.inspection_id AND version=r.inspection_version AND status='cleared' AND is_active;
 IF i.id IS NULL OR NOT EXISTS(SELECT 1 FROM purchase_acceptances WHERE id=i.acceptance_id AND version=i.acceptance_version AND status='accepted' AND is_active) OR NOT EXISTS(SELECT 1 FROM purchase_technical_cases WHERE id=i.technical_id AND version=i.technical_version AND state IN('approved','approved_with_comments') AND is_active) THEN RAISE EXCEPTION 'GRN source evidence is no longer current';END IF;
 expected=y.source_snapshot||jsonb_build_object('receipt_id',y.id,'receipt_version',y.version,'receipt_revision',y.revision,'arrival_date',y.data->'receipt'->>'arrival_date','receiving_officer',y.data->'receipt'->>'receiving_officer','packages_received',y.data->'receipt'->'packages_received','packages_condition',y.data->'receipt'->>'packages_condition','transit_damage_notes',y.data->'receipt'->>'transit_damage_notes','gate_entry_ref',y.data->'receipt'->>'gate_entry_ref');
 IF NEW.source_snapshot IS DISTINCT FROM expected THEN RAISE EXCEPTION 'GRN source must equal its current site receipt';END IF;
 IF NEW.data->>'id' IS DISTINCT FROM NEW.id::text OR NEW.data->>'project_id' IS DISTINCT FROM NEW.project_id::text OR NEW.data->>'package_id' IS DISTINCT FROM NEW.package_id::text OR NEW.data->>'version' IS DISTINCT FROM NEW.version::text OR NEW.data->>'revision' IS DISTINCT FROM NEW.revision::text OR NEW.data->>'is_active' IS DISTINCT FROM NEW.is_active::text OR NEW.data->'source_snapshot' IS DISTINCT FROM NEW.source_snapshot OR NEW.data->'grn'->>'status' IS DISTINCT FROM NEW.status OR (NEW.data->>'created_at')::timestamptz IS DISTINCT FROM NEW.created_at OR (NEW.data->>'updated_at')::timestamptz IS DISTINCT FROM NEW.updated_at OR nullif(NEW.data->>'created_by_id','')::uuid IS DISTINCT FROM NEW.created_by_id OR nullif(NEW.data->>'updated_by_id','')::uuid IS DISTINCT FROM NEW.updated_by_id THEN RAISE EXCEPTION 'GRN projection mismatch';END IF;
 IF NEW.created_by_id IS NULL OR NEW.updated_by_id IS NULL THEN RAISE EXCEPTION 'Use authenticated receiving attribution';END IF;
 d=NEW.data->'grn';
 IF NEW.status='issued' AND (length(trim(coalesce(d->>'grn_number','')))=0 OR coalesce(d->>'grn_date','')='') THEN RAISE EXCEPTION 'Issued GRN needs number and actual date';END IF;
 IF d->>'uom' IS DISTINCT FROM NEW.source_snapshot->>'uom' THEN RAISE EXCEPTION 'GRN uses inherited material unit';END IF;
 IF jsonb_typeof(d->'received_qty') IS DISTINCT FROM 'string' OR jsonb_typeof(d->'accepted_qty') IS DISTINCT FROM 'string' OR jsonb_typeof(d->'rejected_qty') IS DISTINCT FROM 'string' OR coalesce(d->>'received_qty','') !~ '^(0|[1-9][0-9]{0,11})(\.[0-9]{1,6})?$' OR coalesce(d->>'accepted_qty','') !~ '^(0|[1-9][0-9]{0,11})(\.[0-9]{1,6})?$' OR coalesce(d->>'rejected_qty','') !~ '^(0|[1-9][0-9]{0,11})(\.[0-9]{1,6})?$' THEN RAISE EXCEPTION 'Use exact nonnegative GRN decimal quantities';END IF;
 IF NEW.status='issued' AND ((d->>'received_qty')::numeric<=0 OR (d->>'received_qty')::numeric>(NEW.source_snapshot->>'cleared_quantity')::numeric OR (d->>'accepted_qty')::numeric+(d->>'rejected_qty')::numeric>(d->>'received_qty')::numeric) THEN RAISE EXCEPTION 'Issued GRN breakdown must fit actual received and cleared allowance';END IF;
 IF length(coalesce(d->>'grn_number',''))>200 OR length(coalesce(d->>'store_keeper_name',''))>300 OR length(coalesce(d->>'remarks',''))>4000 OR length(coalesce(d->>'warranty_ref',''))>200 OR length(coalesce(d->>'file_name',''))>500 THEN RAISE EXCEPTION 'GRN fields exceed limits';END IF;
 IF coalesce(d->>'grn_date','')<>'' AND ((d->>'grn_date')::date<(y.data->'receipt'->>'arrival_date')::date OR (d->>'grn_date')::date>(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date) THEN RAISE EXCEPTION 'GRN follows actual arrival and cannot be future';END IF;
 IF coalesce(d->>'warranty_valid_until','')<>'' THEN PERFORM (d->>'warranty_valid_until')::date;END IF;
 IF TG_OP='INSERT' AND (NEW.version<>1 OR NEW.revision<>1) THEN RAISE EXCEPTION 'GRN begins at version/revision one';END IF;
 IF TG_OP='UPDATE' THEN
 IF NEW.id<>OLD.id OR NEW.project_id<>OLD.project_id OR NEW.package_id<>OLD.package_id OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'GRN ownership/audit must be retained';END IF;
 IF NEW.revision=OLD.revision THEN
 IF OLD.status='issued' OR NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot THEN RAISE EXCEPTION 'Issued or obsolete GRN needs revision';END IF;
 ELSIF NEW.revision<>OLD.revision+1 OR NEW.status<>'pending' OR d IS DISTINCT FROM jsonb_build_object('status','pending','grn_number','','grn_date','','received_qty','0','accepted_qty','0','rejected_qty','0','uom',NEW.source_snapshot->>'uom','store_keeper_name','','remarks','','warranty_ref','','warranty_valid_until','','file_name','') THEN RAISE EXCEPTION 'GRN revision resets pending and retains prior evidence';END IF;
 END IF;
 RETURN NEW;END $$;
CREATE TRIGGER purchase_grns_guard BEFORE INSERT OR UPDATE ON purchase_grns FOR EACH ROW EXECUTE FUNCTION protect_purchase_grns();
CREATE FUNCTION protect_purchase_grns_history() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'GRN and history cannot be erased';END $$;
CREATE TRIGGER purchase_grns_no_delete BEFORE DELETE ON purchase_grns FOR EACH ROW EXECUTE FUNCTION protect_purchase_grns_history();
CREATE TRIGGER purchase_grns_no_truncate BEFORE TRUNCATE ON purchase_grns FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_grns_history();
CREATE TRIGGER purchase_grns_event_immutable BEFORE UPDATE OR DELETE ON purchase_grns_events FOR EACH ROW EXECUTE FUNCTION protect_purchase_grns_history();
CREATE TRIGGER purchase_grns_event_no_truncate BEFORE TRUNCATE ON purchase_grns_events FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_grns_history();
CREATE FUNCTION check_purchase_grns_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_grns_events WHERE subject_id=NEW.id AND after_snapshot=NEW.data AND created_by_id IS NOT DISTINCT FROM NEW.updated_by_id AND created_at=NEW.updated_at AND action=CASE WHEN TG_OP='UPDATE' AND NEW.revision>OLD.revision THEN 'revise' ELSE 'record' END AND (TG_OP='INSERT' AND before_snapshot='null'::jsonb OR TG_OP='UPDATE' AND before_snapshot=OLD.data)) THEN RAISE EXCEPTION 'GRN mutation requires exact immutable audit';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_grns_write_integrity AFTER INSERT OR UPDATE ON purchase_grns DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_grns_write();
CREATE FUNCTION check_purchase_grns_event() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE p uuid;BEGIN
 SELECT project_id INTO p FROM purchase_grns WHERE id=NEW.subject_id;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_grns' AND entity_type='purchase_grns' AND entity_id=NEW.subject_id AND entity_version=(NEW.after_snapshot->>'version')::integer AND action=NEW.action AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted' AND occurred_at=NEW.created_at) THEN RAISE EXCEPTION 'GRN audit requires central project event';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_grns_event_integrity AFTER INSERT ON purchase_grns_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_grns_event();
COMMIT;
