BEGIN;
CREATE TABLE finance_accounts_acceptance(
 id uuid PRIMARY KEY,project_id uuid NOT NULL REFERENCES projects(id),package_id uuid NOT NULL UNIQUE REFERENCES purchase_work_packages(id),
 engineer_id uuid NOT NULL REFERENCES purchase_engineer_confirmations(id),engineer_version integer NOT NULL CHECK(engineer_version>0),
 version integer NOT NULL CHECK(version>0),revision integer NOT NULL CHECK(revision>0),status varchar(20) NOT NULL CHECK(status IN('pending','accepted')),
 source_snapshot jsonb NOT NULL CHECK(jsonb_typeof(source_snapshot)='object'),data jsonb NOT NULL CHECK(jsonb_typeof(data)='object'),
 is_active boolean NOT NULL DEFAULT true,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id)
);
CREATE TABLE finance_accounts_acceptance_events(
 id uuid PRIMARY KEY,subject_id uuid NOT NULL REFERENCES finance_accounts_acceptance(id),action varchar(20) NOT NULL CHECK(action IN('record','revise')),
 reason text NOT NULL CHECK(length(trim(reason)) BETWEEN 1 AND 2000),before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,created_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id)
);
CREATE INDEX finance_accounts_acceptance_history ON finance_accounts_acceptance_events(subject_id,created_at DESC);
CREATE FUNCTION protect_finance_accounts_acceptance() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE w purchase_engineer_confirmations;o purchase_orders;z purchase_grns;y purchase_site_receipts;x purchase_dispatches;c purchase_client_clearances;q purchase_quality_cases;r purchase_inspection_reports;i purchase_inspections;d jsonb;expected jsonb;BEGIN
 SELECT * INTO w FROM purchase_engineer_confirmations WHERE id=NEW.engineer_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=NEW.engineer_version AND status='confirmed' AND is_active;
 IF w.id IS NULL THEN RAISE EXCEPTION 'Accounts acceptance needs current confirmed engineer evidence';END IF;
 SELECT * INTO z FROM purchase_grns WHERE id=w.grn_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=w.grn_version AND status='issued' AND is_active;
 IF z.id IS NULL THEN RAISE EXCEPTION 'Accounts acceptance needs current issued GRN';END IF;
 SELECT * INTO o FROM purchase_orders WHERE id=(w.source_snapshot->>'order_id')::uuid AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=(w.source_snapshot->>'order_version')::integer AND state='released' AND is_active;
 IF o.id IS NULL THEN RAISE EXCEPTION 'Accounts acceptance needs its actual released PO';END IF;
 SELECT * INTO y FROM purchase_site_receipts WHERE id=z.receipt_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=z.receipt_version AND status='delivered_at_site' AND is_active;
 IF y.id IS NULL THEN RAISE EXCEPTION 'Accounts acceptance needs the current delivered site receipt';END IF;
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
 expected=w.source_snapshot||jsonb_build_object('engineer_id',w.id,'engineer_version',w.version,'engineer_revision',w.revision,'confirmation_date',w.data->'confirmation'->>'confirmation_date','engineer_name',w.data->'confirmation'->>'engineer_name','designation',w.data->'confirmation'->>'designation','mbook_ref',w.data->'confirmation'->>'mbook_ref','work_package_ref',w.data->'confirmation'->>'work_package_ref','engineer_remarks',w.data->'confirmation'->>'remarks','currency',o.source_snapshot->>'currency');
 IF NEW.source_snapshot IS DISTINCT FROM expected THEN RAISE EXCEPTION 'Accounts acceptance source must equal current engineer and released PO';END IF;
 IF NEW.data->>'id' IS DISTINCT FROM NEW.id::text OR NEW.data->>'project_id' IS DISTINCT FROM NEW.project_id::text OR NEW.data->>'package_id' IS DISTINCT FROM NEW.package_id::text OR NEW.data->>'version' IS DISTINCT FROM NEW.version::text OR NEW.data->>'revision' IS DISTINCT FROM NEW.revision::text OR NEW.data->>'is_active' IS DISTINCT FROM NEW.is_active::text OR NEW.data->'source_snapshot' IS DISTINCT FROM NEW.source_snapshot OR NEW.data->'acceptance'->>'status' IS DISTINCT FROM NEW.status OR (NEW.data->>'created_at')::timestamptz IS DISTINCT FROM NEW.created_at OR (NEW.data->>'updated_at')::timestamptz IS DISTINCT FROM NEW.updated_at OR nullif(NEW.data->>'created_by_id','')::uuid IS DISTINCT FROM NEW.created_by_id OR nullif(NEW.data->>'updated_by_id','')::uuid IS DISTINCT FROM NEW.updated_by_id THEN RAISE EXCEPTION 'Accounts acceptance projection mismatch';END IF;
 IF NEW.created_by_id IS NULL OR NEW.updated_by_id IS NULL THEN RAISE EXCEPTION 'Use authenticated Finance attribution';END IF;
 d=NEW.data->'acceptance';
 IF EXISTS(SELECT 1 FROM unnest(ARRAY['status','acceptance_date','officer_name','voucher_ref','booked_amount','notes']) k WHERE jsonb_typeof(d->k) IS DISTINCT FROM 'string') OR (SELECT count(*) FROM jsonb_object_keys(d))<>6 THEN RAISE EXCEPTION 'Use declared Accounts fields as strings';END IF;
 IF NEW.status='accepted' AND (length(trim(d->>'officer_name'))=0 OR d->>'acceptance_date'='') THEN RAISE EXCEPTION 'Accepted needs actual officer and date';END IF;
 IF length(d->>'officer_name')>300 OR length(d->>'voucher_ref')>200 OR length(d->>'notes')>4000 OR d->>'booked_amount' !~ '^(0|[1-9][0-9]{0,11})(\.[0-9]{1,2})?$' THEN RAISE EXCEPTION 'Accounts fields exceed limits';END IF;
 IF d->>'acceptance_date'<>'' AND ((d->>'acceptance_date')::date<(w.data->'confirmation'->>'confirmation_date')::date OR (d->>'acceptance_date')::date>(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date) THEN RAISE EXCEPTION 'Acceptance follows engineer confirmation and cannot be future';END IF;
 IF TG_OP='INSERT' AND (NEW.version<>1 OR NEW.revision<>1) THEN RAISE EXCEPTION 'Accounts acceptance begins at version/revision one';END IF;
 IF TG_OP='UPDATE' THEN
 IF NEW.id<>OLD.id OR NEW.project_id<>OLD.project_id OR NEW.package_id<>OLD.package_id OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'Accounts acceptance ownership/audit must be retained';END IF;
 IF NEW.revision=OLD.revision THEN
 IF OLD.status='accepted' OR NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot THEN RAISE EXCEPTION 'Accepted or obsolete Accounts evidence needs revision';END IF;
 ELSIF NEW.revision<>OLD.revision+1 OR NEW.status<>'pending' OR d IS DISTINCT FROM jsonb_build_object('status','pending','acceptance_date','','officer_name','','voucher_ref','','booked_amount','0','notes','') THEN RAISE EXCEPTION 'Accounts acceptance revision resets pending and retains prior evidence';END IF;
 END IF;
 RETURN NEW;END $$;
CREATE TRIGGER finance_accounts_acceptance_guard BEFORE INSERT OR UPDATE ON finance_accounts_acceptance FOR EACH ROW EXECUTE FUNCTION protect_finance_accounts_acceptance();
CREATE FUNCTION protect_finance_accounts_acceptance_history() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Accounts acceptance and history cannot be erased';END $$;
CREATE TRIGGER finance_accounts_acceptance_no_delete BEFORE DELETE ON finance_accounts_acceptance FOR EACH ROW EXECUTE FUNCTION protect_finance_accounts_acceptance_history();
CREATE TRIGGER finance_accounts_acceptance_no_truncate BEFORE TRUNCATE ON finance_accounts_acceptance FOR EACH STATEMENT EXECUTE FUNCTION protect_finance_accounts_acceptance_history();
CREATE TRIGGER finance_accounts_acceptance_event_immutable BEFORE UPDATE OR DELETE ON finance_accounts_acceptance_events FOR EACH ROW EXECUTE FUNCTION protect_finance_accounts_acceptance_history();
CREATE TRIGGER finance_accounts_acceptance_event_no_truncate BEFORE TRUNCATE ON finance_accounts_acceptance_events FOR EACH STATEMENT EXECUTE FUNCTION protect_finance_accounts_acceptance_history();
CREATE FUNCTION check_finance_accounts_acceptance_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM finance_accounts_acceptance_events WHERE subject_id=NEW.id AND after_snapshot=NEW.data AND created_by_id IS NOT DISTINCT FROM NEW.updated_by_id AND created_at=NEW.updated_at AND action=CASE WHEN TG_OP='UPDATE' AND NEW.revision>OLD.revision THEN 'revise' ELSE 'record' END AND (TG_OP='INSERT' AND before_snapshot='null'::jsonb OR TG_OP='UPDATE' AND before_snapshot=OLD.data)) THEN RAISE EXCEPTION 'Accounts acceptance mutation requires exact immutable audit';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER finance_accounts_acceptance_write_integrity AFTER INSERT OR UPDATE ON finance_accounts_acceptance DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_finance_accounts_acceptance_write();
CREATE FUNCTION check_finance_accounts_acceptance_event() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE p uuid;financial boolean;kind text;act text;BEGIN
 SELECT project_id INTO p FROM finance_accounts_acceptance WHERE id=NEW.subject_id;
 financial=NEW.after_snapshot->'acceptance'->>'status'='accepted' OR NEW.action='revise' AND NEW.before_snapshot->'acceptance'->>'status'='accepted';
 kind=CASE WHEN financial THEN 'financial_decision' ELSE 'accounts_acceptance' END;
 act=CASE WHEN NEW.after_snapshot->'acceptance'->>'status'='accepted' THEN 'approved' WHEN financial THEN 'returned' ELSE NEW.action END;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='finance_accounts_acceptance' AND entity_type=kind AND entity_id=NEW.subject_id AND entity_version=(NEW.after_snapshot->>'version')::integer AND action=act AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted' AND occurred_at=NEW.created_at) THEN RAISE EXCEPTION 'Accounts audit requires its exact central event';END IF;
 IF financial AND NOT EXISTS(SELECT 1 FROM communication_notices WHERE source_event_id=NEW.id AND decision_id=NEW.subject_id AND source_version=(NEW.after_snapshot->>'version')::integer AND decision_action=act AND project_id=p AND created_by_id=NEW.created_by_id) THEN RAISE EXCEPTION 'Accounts decision requires atomic Director notice';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER finance_accounts_acceptance_event_integrity AFTER INSERT ON finance_accounts_acceptance_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_finance_accounts_acceptance_event();
CREATE OR REPLACE FUNCTION protect_communication_notice() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF TG_OP='INSERT' THEN
  IF NEW.version<>1 OR NEW.read_at IS NOT NULL OR NEW.acknowledged_at IS NOT NULL OR NEW.created_at<>NEW.updated_at OR NEW.created_by_id IS DISTINCT FROM NEW.updated_by_id THEN RAISE EXCEPTION 'New notice must be unread with source attribution';END IF;
  IF NOT EXISTS(SELECT 1 FROM business_events e WHERE e.id=NEW.source_event_id AND e.source_module IN('finance_approvals','finance_accounts_acceptance') AND e.entity_type='financial_decision' AND e.entity_id=NEW.decision_id AND e.entity_version=NEW.source_version AND e.action=NEW.decision_action AND e.actor_id=NEW.created_by_id AND e.actor_kind='user' AND e.scope_kind='project' AND e.scope_id=NEW.project_id AND e.visibility='restricted' AND e.occurred_at=NEW.created_at) THEN RAISE EXCEPTION 'Notice source must be its recorded financial decision';END IF;
  IF NOT EXISTS(SELECT 1 FROM organisation_director_designation d JOIN users u ON u.id=d.user_id WHERE d.id=NEW.director_designation_id AND d.version=NEW.director_version AND d.user_id=NEW.recipient_id AND d.is_active AND u.is_active) THEN RAISE EXCEPTION 'Notice requires the effective Director';END IF;
 ELSE
  IF ROW(NEW.id,NEW.source_event_id,NEW.recipient_id,NEW.director_designation_id,NEW.director_version,NEW.decision_id,NEW.project_id,NEW.source_version,NEW.decision_action,NEW.is_active,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.id,OLD.source_event_id,OLD.recipient_id,OLD.director_designation_id,OLD.director_version,OLD.decision_id,OLD.project_id,OLD.source_version,OLD.decision_action,OLD.is_active,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Notice identity and addressed source are immutable';END IF;
  IF NEW.version<>OLD.version+1 OR NEW.updated_by_id IS DISTINCT FROM NEW.recipient_id OR NEW.updated_at<OLD.updated_at OR (OLD.read_at IS NOT NULL AND NEW.read_at IS DISTINCT FROM OLD.read_at) OR (OLD.acknowledged_at IS NOT NULL AND NEW.acknowledged_at IS DISTINCT FROM OLD.acknowledged_at) OR ROW(NEW.read_at,NEW.acknowledged_at) IS NOT DISTINCT FROM ROW(OLD.read_at,OLD.acknowledged_at) THEN RAISE EXCEPTION 'Notice receipts must advance monotonically for the recipient';END IF;
 END IF;RETURN NEW;END $$;
COMMIT;
