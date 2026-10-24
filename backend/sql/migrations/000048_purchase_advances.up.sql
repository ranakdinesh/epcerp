BEGIN;
CREATE TABLE purchase_advances(
 id uuid PRIMARY KEY,project_id uuid NOT NULL REFERENCES projects(id),package_id uuid NOT NULL UNIQUE REFERENCES purchase_work_packages(id),
 order_id uuid NOT NULL REFERENCES purchase_orders(id),order_version integer NOT NULL CHECK(order_version>1),
 version integer NOT NULL CHECK(version>0),state varchar(24) NOT NULL CHECK(state IN('draft','requested','approved','rejected','not_applicable','disbursed')),
 source_snapshot jsonb NOT NULL CHECK(jsonb_typeof(source_snapshot)='object'),data jsonb NOT NULL CHECK(jsonb_typeof(data)='object'),
 submitted_snapshot text NOT NULL,source_fingerprint varchar(64) NOT NULL,
 decision_id uuid,is_active boolean NOT NULL CHECK(is_active),created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,created_by_id uuid NOT NULL REFERENCES users(id),updated_by_id uuid NOT NULL REFERENCES users(id),
 UNIQUE(id,project_id,package_id)
);
CREATE TABLE purchase_advance_events(
 id uuid PRIMARY KEY,subject_id uuid NOT NULL REFERENCES purchase_advances(id),action varchar(20) NOT NULL CHECK(action IN('prepare','submit','revise','approve','reject','disburse')),
 reason varchar(2000) NOT NULL CHECK(length(trim(reason))>0),before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL,created_by_id uuid NOT NULL REFERENCES users(id)
);
CREATE UNIQUE INDEX purchase_advance_event_version ON purchase_advance_events(subject_id,(after_snapshot->>'version'));
CREATE INDEX purchase_advance_history ON purchase_advance_events(subject_id,created_at DESC);
CREATE TABLE finance_advance_decisions(
 id uuid PRIMARY KEY,project_id uuid NOT NULL,package_id uuid NOT NULL,request_id uuid NOT NULL,order_id uuid NOT NULL REFERENCES purchase_orders(id),order_version integer NOT NULL CHECK(order_version>1),
 source_version integer NOT NULL CHECK(source_version>1),submitted_by_id uuid NOT NULL REFERENCES users(id),outcome varchar(20) NOT NULL CHECK(outcome IN('approved','rejected')),
 kind varchar(24) NOT NULL CHECK(kind IN('payment','not_applicable')),amount numeric(16,2) NOT NULL CHECK(amount>=0),currency char(3) NOT NULL CHECK(currency~'^[A-Z]{3}$'),
 source_fingerprint char(64) NOT NULL CHECK(source_fingerprint~'^[0-9a-f]{64}$'),reason varchar(2000) NOT NULL CHECK(length(trim(reason))>0),
 is_active boolean NOT NULL CHECK(is_active),created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,created_by_id uuid NOT NULL REFERENCES users(id),updated_by_id uuid NOT NULL REFERENCES users(id),
 CHECK(created_by_id<>submitted_by_id),CHECK(updated_at=created_at),CHECK(updated_by_id=created_by_id),CHECK((kind='payment' AND amount>0) OR (kind='not_applicable' AND amount=0)),
 UNIQUE(request_id,source_version),FOREIGN KEY(request_id,project_id,package_id) REFERENCES purchase_advances(id,project_id,package_id),
 FOREIGN KEY(id) REFERENCES business_events(id) DEFERRABLE INITIALLY DEFERRED
);
ALTER TABLE purchase_advances ADD CONSTRAINT purchase_advance_decision_fk FOREIGN KEY(decision_id) REFERENCES finance_advance_decisions(id);
CREATE TRIGGER finance_advance_immutable BEFORE UPDATE OR DELETE ON finance_advance_decisions FOR EACH ROW EXECUTE FUNCTION protect_finance_budget_decisions();
CREATE TRIGGER finance_advance_no_truncate BEFORE TRUNCATE ON finance_advance_decisions FOR EACH STATEMENT EXECUTE FUNCTION protect_finance_budget_decisions();
CREATE FUNCTION protect_purchase_advance() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE o purchase_orders;a purchase_acceptances;d jsonb;s jsonb;expected jsonb; pct numeric;calculated numeric;
BEGIN
 d=NEW.data;s=NEW.source_snapshot;
 SELECT * INTO o FROM purchase_orders WHERE id=NEW.order_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=NEW.order_version AND state='released' AND is_active;
 SELECT * INTO a FROM purchase_acceptances WHERE id=(s->>'acceptance_id')::uuid AND project_id=NEW.project_id AND package_id=NEW.package_id AND order_id=NEW.order_id AND version=(s->>'acceptance_version')::integer AND status='accepted' AND is_active;
 IF o.id IS NULL OR a.id IS NULL THEN RAISE EXCEPTION 'Advance requires its actual released and unconditionally accepted PO';END IF;
 expected=jsonb_build_object('project_id',o.project_id,'package_id',o.package_id,'order_id',o.id,'order_version',o.version,'number',o.number,'date',o.data->'details'->>'date',
 'supplier_id',o.source_snapshot->'supplier'->>'id','supplier_name',o.source_snapshot->'supplier'->>'name','currency',o.source_snapshot->>'currency','total',o.source_snapshot->>'total','payment_terms',o.source_snapshot->>'payment_terms',
 'acceptance_id',a.id,'acceptance_version',a.version,'calculation_basis','po_total');
 IF s-ARRAY['supplier_verification','available','blockers'] IS DISTINCT FROM expected OR s->>'available' IS DISTINCT FROM 'true' OR s->'blockers' IS DISTINCT FROM '[]'::jsonb OR coalesce(s->>'supplier_verification','')='' THEN RAISE EXCEPTION 'Advance source projection mismatch';END IF;
 IF d->>'id' IS DISTINCT FROM NEW.id::text OR d->>'project_id' IS DISTINCT FROM NEW.project_id::text OR d->>'package_id' IS DISTINCT FROM NEW.package_id::text OR d->>'version' IS DISTINCT FROM NEW.version::text OR d->>'state' IS DISTINCT FROM NEW.state OR d->'source_snapshot' IS DISTINCT FROM s OR d->>'is_active' IS DISTINCT FROM NEW.is_active::text OR d->>'created_by_id' IS DISTINCT FROM NEW.created_by_id::text OR d->>'updated_by_id' IS DISTINCT FROM NEW.updated_by_id::text OR (d->>'created_at')::timestamptz IS DISTINCT FROM NEW.created_at OR (d->>'updated_at')::timestamptz IS DISTINCT FROM NEW.updated_at OR nullif(d->>'decision_id','')::uuid IS DISTINCT FROM NEW.decision_id THEN RAISE EXCEPTION 'Advance record projection mismatch';END IF;
 IF d->'details'->>'kind' NOT IN('payment','not_applicable') OR d->'details'->>'kind' IS NULL THEN RAISE EXCEPTION 'Declare the advance disposition';END IF;
 IF coalesce(d->'details'->>'advance_pct','')<>'' THEN
  IF d->'details'->>'advance_pct' !~ '^(0|[1-9][0-9]{0,13})([.][0-9]{1,2})?$' THEN RAISE EXCEPTION 'Use exact advance percentage';END IF;
  pct=(d->'details'->>'advance_pct')::numeric;calculated=round((s->>'total')::numeric*pct/100,2);
  IF pct>100 OR d->>'amount' IS DISTINCT FROM calculated::text THEN RAISE EXCEPTION 'Advance amount must equal the exact PO percentage';END IF;
 ELSE IF NEW.state<>'draft' OR coalesce(d->>'amount','')<>'' THEN RAISE EXCEPTION 'Submitted advance requires exact amount';END IF;END IF;
 IF d->'details'->>'kind'='not_applicable' AND (coalesce(pct,0)<>0 OR coalesce(d->'details'->>'proforma_invoice_ref','')<>'' OR coalesce(d->'details'->>'proforma_invoice_date','')<>'' OR coalesce(d->'details'->>'bank_account_ref','')<>'') THEN RAISE EXCEPTION 'No-advance disposition carries no payment instructions';END IF;
 IF NEW.state='draft' THEN
  IF NEW.decision_id IS NOT NULL OR coalesce(d->>'submitted_by_id','')<>'' OR d->'submitted_at'<>'null'::jsonb OR coalesce((d->>'submitted_version')::integer,0)<>0 OR coalesce(d->>'reviewed_by_id','')<>'' OR d->'reviewed_at'<>'null'::jsonb OR d->'payment'<>'null'::jsonb OR NEW.submitted_snapshot<>'' OR NEW.source_fingerprint<>'' THEN RAISE EXCEPTION 'Draft retains no effective approval or payment';END IF;
 ELSE
  expected=jsonb_build_object('Source',s-ARRAY['available','blockers'],'Details',d->'details','Amount',d->>'amount','SubmittedBy',d->>'submitted_by_id','SubmittedVersion',(d->>'submitted_version')::integer,'SubmittedAt',d->'submitted_at');
  IF NEW.submitted_snapshot='' OR NEW.submitted_snapshot::jsonb IS DISTINCT FROM expected OR NEW.source_fingerprint IS DISTINCT FROM encode(sha256(convert_to(NEW.submitted_snapshot,'UTF8')),'hex') THEN RAISE EXCEPTION 'Submitted advance fingerprint must match exact saved source';END IF;
  IF coalesce((d->>'submitted_version')::integer,0)<2 OR coalesce(d->>'submitted_by_id','')='' OR d->'submitted_at'='null'::jsonb OR length(trim(coalesce(d->'details'->>'request_no','')))=0 OR coalesce(d->'details'->>'request_date','')='' OR (d->'details'->>'request_date')::date<(s->>'date')::date THEN RAISE EXCEPTION 'Complete actual advance submission required';END IF;
  IF d->'details'->>'kind'='payment' AND (calculated<=0 OR length(trim(coalesce(d->'details'->>'proforma_invoice_ref','')))=0 OR coalesce(d->'details'->>'proforma_invoice_date','')='' OR (d->'details'->>'proforma_invoice_date')::date>(d->'details'->>'request_date')::date OR length(trim(coalesce(d->'details'->>'bank_account_ref','')))=0) THEN RAISE EXCEPTION 'Payment request requires positive amount and PI/bank references';END IF;
  IF d->'details'->>'kind'='not_applicable' AND length(trim(coalesce(d->'details'->>'notes','')))=0 THEN RAISE EXCEPTION 'No advance requires its reason';END IF;
 END IF;
 IF NEW.state='requested' AND (NEW.decision_id IS NOT NULL OR coalesce(d->>'reviewed_by_id','')<>'' OR d->'reviewed_at'<>'null'::jsonb OR d->'payment'<>'null'::jsonb) THEN RAISE EXCEPTION 'Requested advance has no decision/payment';END IF;
 IF NEW.state<>'disbursed' AND d->'payment' IS DISTINCT FROM 'null'::jsonb THEN RAISE EXCEPTION 'Only disbursed advance carries payment evidence';END IF;
 IF NEW.state='disbursed' AND (d->'details'->>'kind'<>'payment' OR length(trim(coalesce(d->'payment'->>'voucher_ref','')))=0 OR coalesce(d->'payment'->>'disbursed_date','')='' OR (d->'payment'->>'disbursed_date')::date<((d->>'reviewed_at')::timestamptz AT TIME ZONE 'Asia/Kolkata')::date) THEN
  RAISE EXCEPTION 'Payment requires its actual voucher and approval date';END IF;
 IF TG_OP='INSERT' THEN
  IF NEW.version<>1 OR NEW.state<>'draft' OR NEW.created_at<>NEW.updated_at OR NEW.created_by_id<>NEW.updated_by_id THEN RAISE EXCEPTION 'Advance starts as an attributed draft';END IF;
 ELSE
  IF ROW(NEW.id,NEW.project_id,NEW.package_id,NEW.order_id,NEW.order_version,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.id,OLD.project_id,OLD.package_id,OLD.order_id,OLD.order_version,OLD.created_at,OLD.created_by_id) OR NEW.version<>OLD.version+1 OR NEW.updated_at<OLD.updated_at OR OLD.state='disbursed' THEN RAISE EXCEPTION 'Advance identity/history and completed payment are immutable';END IF;
  IF NOT (OLD.state='draft' AND NEW.state IN('draft','requested') OR OLD.state='requested' AND NEW.state IN('approved','rejected','not_applicable','draft') OR OLD.state IN('approved','rejected','not_applicable') AND NEW.state='draft' OR OLD.state='approved' AND NEW.state='disbursed') THEN RAISE EXCEPTION 'Invalid advance transition';END IF;
  IF NEW.state<>'draft' AND OLD.state<>'draft' AND (NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot OR NEW.submitted_snapshot IS DISTINCT FROM OLD.submitted_snapshot OR NEW.source_fingerprint IS DISTINCT FROM OLD.source_fingerprint) THEN RAISE EXCEPTION 'Submitted source cannot be changed during decision/payment';END IF;
  IF OLD.state='draft' AND NEW.state='requested' AND ((d->>'submitted_version')::integer<>NEW.version OR d->>'submitted_by_id' IS DISTINCT FROM NEW.updated_by_id::text OR (d->>'submitted_at')::timestamptz IS DISTINCT FROM NEW.updated_at) THEN RAISE EXCEPTION 'Submission binds current version and actor';END IF;
  IF OLD.state='approved' AND NEW.state='disbursed' AND (NEW.decision_id IS DISTINCT FROM OLD.decision_id OR d->'reviewed_at' IS DISTINCT FROM OLD.data->'reviewed_at' OR d->'reviewed_by_id' IS DISTINCT FROM OLD.data->'reviewed_by_id') THEN RAISE EXCEPTION 'Payment retains its exact Finance decision';END IF;
 END IF;
 RETURN NEW;END $$;
CREATE TRIGGER purchase_advance_guard BEFORE INSERT OR UPDATE ON purchase_advances FOR EACH ROW EXECUTE FUNCTION protect_purchase_advance();
CREATE FUNCTION protect_purchase_advance_history() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Advance records and history cannot be erased';END $$;
CREATE TRIGGER purchase_advance_no_delete BEFORE DELETE ON purchase_advances FOR EACH ROW EXECUTE FUNCTION protect_purchase_advance_history();
CREATE TRIGGER purchase_advance_no_truncate BEFORE TRUNCATE ON purchase_advances FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_advance_history();
CREATE TRIGGER purchase_advance_event_immutable BEFORE UPDATE OR DELETE ON purchase_advance_events FOR EACH ROW EXECUTE FUNCTION protect_purchase_advance_history();
CREATE TRIGGER purchase_advance_event_no_truncate BEFORE TRUNCATE ON purchase_advance_events FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_advance_history();
CREATE FUNCTION check_purchase_advance_write() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE e purchase_advance_events;f finance_advance_decisions;BEGIN
 SELECT * INTO e FROM purchase_advance_events WHERE subject_id=NEW.id AND after_snapshot=NEW.data AND created_by_id=NEW.updated_by_id AND created_at=NEW.updated_at;
 IF e.id IS NULL OR (TG_OP='INSERT' AND (e.before_snapshot<>'null'::jsonb OR e.action<>'prepare')) OR (TG_OP='UPDATE' AND e.before_snapshot IS DISTINCT FROM OLD.data) THEN RAISE EXCEPTION 'Advance mutation requires exact immutable audit';END IF;
 IF (e.action='prepare' AND NEW.state<>'draft') OR (e.action='submit' AND NEW.state<>'requested') OR (e.action='revise' AND NEW.state<>'draft') OR (e.action='approve' AND NEW.state NOT IN('approved','not_applicable')) OR (e.action='reject' AND NEW.state<>'rejected') OR (e.action='disburse' AND NEW.state<>'disbursed') THEN RAISE EXCEPTION 'Advance audit action differs from resulting state';END IF;
 IF NEW.state IN('approved','rejected','not_applicable','disbursed') THEN
  SELECT * INTO f FROM finance_advance_decisions WHERE id=NEW.decision_id;
  IF f.id IS NULL OR ROW(f.request_id,f.project_id,f.package_id,f.order_id,f.order_version,f.source_version,f.source_fingerprint) IS DISTINCT FROM ROW(NEW.id,NEW.project_id,NEW.package_id,NEW.order_id,NEW.order_version,(NEW.data->>'submitted_version')::integer,NEW.source_fingerprint::char(64)) OR f.amount IS DISTINCT FROM (NEW.data->>'amount')::numeric OR f.currency IS DISTINCT FROM NEW.source_snapshot->>'currency' OR f.kind IS DISTINCT FROM NEW.data->'details'->>'kind' OR f.submitted_by_id::text IS DISTINCT FROM NEW.data->>'submitted_by_id' OR f.created_by_id::text IS DISTINCT FROM NEW.data->>'reviewed_by_id' OR NEW.data->'reviewed_at'='null'::jsonb OR (NEW.state='rejected') IS DISTINCT FROM (f.outcome='rejected') OR (NEW.state='not_applicable') IS DISTINCT FROM (f.kind='not_applicable' AND f.outcome='approved') THEN RAISE EXCEPTION 'Advance outcome requires its matching Finance decision';END IF;
 END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_advance_write_integrity AFTER INSERT OR UPDATE ON purchase_advances DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_advance_write();
CREATE FUNCTION check_purchase_advance_event() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE p uuid;BEGIN
 SELECT project_id INTO p FROM purchase_advances WHERE id=NEW.subject_id;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_advances' AND entity_type='purchase_advance' AND entity_id=NEW.subject_id AND entity_version=(NEW.after_snapshot->>'version')::integer AND action=NEW.action AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted' AND occurred_at=NEW.created_at) THEN RAISE EXCEPTION 'Advance audit requires central project event';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_advance_event_integrity AFTER INSERT ON purchase_advance_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_advance_event();
CREATE FUNCTION protect_finance_advance_source() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE r purchase_advances;BEGIN
 SELECT * INTO r FROM purchase_advances WHERE id=NEW.request_id FOR UPDATE;
 IF r.id IS NULL OR r.state<>'requested' OR ROW(r.project_id,r.package_id,r.order_id,r.order_version) IS DISTINCT FROM ROW(NEW.project_id,NEW.package_id,NEW.order_id,NEW.order_version) OR r.version<>NEW.source_version OR r.source_fingerprint IS DISTINCT FROM NEW.source_fingerprint OR (r.data->>'amount')::numeric IS DISTINCT FROM NEW.amount OR r.source_snapshot->>'currency' IS DISTINCT FROM NEW.currency OR r.data->>'submitted_by_id' IS DISTINCT FROM NEW.submitted_by_id::text OR r.data->'details'->>'kind' IS DISTINCT FROM NEW.kind THEN RAISE EXCEPTION 'Finance reviews the exact current submitted advance';END IF;
 RETURN NEW;END $$;
CREATE TRIGGER finance_advance_source BEFORE INSERT ON finance_advance_decisions FOR EACH ROW EXECUTE FUNCTION protect_finance_advance_source();
CREATE FUNCTION check_finance_advance_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='finance_approvals' AND entity_type='financial_decision' AND entity_id=NEW.id AND entity_version=1 AND action=NEW.outcome AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=NEW.project_id AND visibility='restricted' AND occurred_at=NEW.created_at) OR NOT EXISTS(SELECT 1 FROM communication_notices WHERE source_event_id=NEW.id AND decision_id=NEW.id AND decision_action=NEW.outcome AND project_id=NEW.project_id AND created_by_id=NEW.created_by_id) THEN RAISE EXCEPTION 'Advance Finance decision requires its immutable project event and Director notice';END IF;
 IF NOT EXISTS(SELECT 1 FROM purchase_advance_events WHERE subject_id=NEW.request_id AND after_snapshot->>'decision_id'=NEW.id::text AND (after_snapshot->>'submitted_version')::integer=NEW.source_version AND created_by_id=NEW.created_by_id AND action=CASE NEW.outcome WHEN 'approved' THEN 'approve' ELSE 'reject' END) THEN RAISE EXCEPTION 'Finance decision requires the consuming advance decision audit';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER finance_advance_event AFTER INSERT ON finance_advance_decisions DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_finance_advance_event();
COMMIT;
