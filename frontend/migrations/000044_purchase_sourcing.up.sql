BEGIN;
CREATE TABLE purchase_sourcing_cases(
 id uuid PRIMARY KEY,project_id uuid NOT NULL REFERENCES projects(id),package_id uuid NOT NULL UNIQUE REFERENCES purchase_work_packages(id),approver_id uuid NOT NULL REFERENCES users(id),version integer NOT NULL CHECK(version>0),state varchar(20) NOT NULL CHECK(state IN('draft','pending','approved','released')),is_active boolean NOT NULL DEFAULT true,
 source_snapshot jsonb NOT NULL CHECK(jsonb_typeof(source_snapshot)='object'),data jsonb NOT NULL CHECK(jsonb_typeof(data)='object'),created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id),UNIQUE(id,project_id)
);
CREATE INDEX purchase_sourcing_project_scope ON purchase_sourcing_cases(project_id,package_id);
CREATE FUNCTION protect_purchase_sourcing() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE o jsonb;n jsonb;BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_work_packages p WHERE p.id=NEW.package_id AND p.project_id=NEW.project_id) THEN RAISE EXCEPTION 'Sourcing package belongs to another project';END IF;
 IF NEW.data->>'id' IS DISTINCT FROM NEW.id::text OR NEW.data->>'project_id' IS DISTINCT FROM NEW.project_id::text OR NEW.data->>'package_id' IS DISTINCT FROM NEW.package_id::text OR NEW.data->>'approver_id' IS DISTINCT FROM NEW.approver_id::text OR NEW.data->>'state' IS DISTINCT FROM NEW.state OR NEW.data->>'version' IS DISTINCT FROM NEW.version::text OR NEW.data->'source_snapshot' IS DISTINCT FROM NEW.source_snapshot OR NEW.data->>'is_active' IS DISTINCT FROM NEW.is_active::text THEN RAISE EXCEPTION 'Sourcing record projection does not match owned state';END IF;
 IF jsonb_typeof(NEW.data->'offers') IS DISTINCT FROM 'array' OR jsonb_array_length(NEW.data->'offers')>50 THEN RAISE EXCEPTION 'Sourcing offers must be a bounded array';END IF;
 FOR o IN SELECT value FROM jsonb_array_elements(NEW.data->'offers') LOOP
  IF jsonb_typeof(o) IS DISTINCT FROM 'object' OR coalesce(o->>'id','') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$' OR coalesce(o->>'state','') NOT IN('draft','sent','no_bid','quoted') OR o->>'is_active' IS DISTINCT FROM 'true' OR NOT EXISTS(SELECT 1 FROM suppliers WHERE id::text=o->>'supplier_id') THEN RAISE EXCEPTION 'Sourcing offers require owned identity, supplier and state';END IF;
 END LOOP;
 IF EXISTS(SELECT 1 FROM jsonb_array_elements(NEW.data->'offers') GROUP BY value->>'id' HAVING count(*)>1) OR EXISTS(SELECT 1 FROM jsonb_array_elements(NEW.data->'offers') GROUP BY value->>'supplier_id' HAVING count(*)>1) THEN RAISE EXCEPTION 'Sourcing offers must have unique identities and suppliers';END IF;
 IF NEW.state IN('pending','approved','released') AND (jsonb_typeof(NEW.data->'decision_snapshot') IS DISTINCT FROM 'object' OR NEW.data->>'selected_offer_id' IS NULL OR NEW.data->>'selected_offer_id'='' OR NEW.data->>'proposed_by_id' IS NULL OR NEW.data->>'proposed_by_id'='' OR NEW.data->>'proposed_by_id'=NEW.approver_id::text) THEN RAISE EXCEPTION 'Independent sourcing selection evidence is required';END IF;
 IF NEW.state IN('pending','approved','released') AND (NEW.data->'decision_snapshot'->'source' IS DISTINCT FROM NEW.source_snapshot OR NEW.data->'decision_snapshot'->'terms' IS DISTINCT FROM NEW.data->'loi_terms' OR coalesce(NEW.data->'decision_snapshot'->>'financial_reference','')='' OR NOT EXISTS(SELECT 1 FROM jsonb_array_elements(NEW.data->'offers') WHERE value->>'id'=NEW.data->>'selected_offer_id' AND value=NEW.data->'decision_snapshot'->'offer' AND value->>'state'='quoted' AND value->>'compliant'='true')) THEN RAISE EXCEPTION 'Selection must capture its exact source, offer, terms and Finance evidence';END IF;
 IF NEW.state IN('approved','released') AND (NEW.data->>'approved_by_id' IS DISTINCT FROM NEW.approver_id::text OR NEW.data->>'approved_at' IS NULL OR NEW.data->>'approved_by_id'=NEW.data->>'proposed_by_id') THEN RAISE EXCEPTION 'Independent sourcing approval is required';END IF;
 IF NEW.state='released' AND (coalesce(NEW.data->>'loi_reference','')='' OR NEW.data->>'released_at' IS NULL OR coalesce(NEW.data->>'loi_dispatch_reference','')='') THEN RAISE EXCEPTION 'Durable LOI and dispatch evidence are required';END IF;
 IF TG_OP='INSERT' AND NEW.state<>'draft' THEN RAISE EXCEPTION 'Sourcing must begin in draft';END IF;
 IF TG_OP='UPDATE' THEN
  IF NOT ((OLD.state='draft' AND NEW.state IN('draft','pending')) OR (OLD.state='pending' AND NEW.state IN('draft','approved')) OR (OLD.state='approved' AND NEW.state IN('draft','released'))) THEN RAISE EXCEPTION 'Invalid sourcing transition';END IF;
  IF OLD.state='released' THEN RAISE EXCEPTION 'Released LOI is immutable';END IF;
  IF NEW.id IS DISTINCT FROM OLD.id OR NEW.project_id IS DISTINCT FROM OLD.project_id OR NEW.package_id IS DISTINCT FROM OLD.package_id OR NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot OR NEW.created_at IS DISTINCT FROM OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'Sourcing ownership/source/version is immutable';END IF;
  IF OLD.state IN('pending','approved') AND NEW.state<>'draft' AND (NEW.data->'decision_snapshot' IS DISTINCT FROM OLD.data->'decision_snapshot' OR NEW.data->'offers' IS DISTINCT FROM OLD.data->'offers' OR NEW.approver_id IS DISTINCT FROM OLD.approver_id) THEN RAISE EXCEPTION 'Submitted sourcing evidence is immutable';END IF;
  FOR o IN SELECT value FROM jsonb_array_elements(OLD.data->'offers') LOOP
   SELECT value INTO n FROM jsonb_array_elements(NEW.data->'offers') WHERE value->>'id'=o->>'id';
   IF NOT FOUND OR n->>'supplier_id' IS DISTINCT FROM o->>'supplier_id' OR n->>'created_at' IS DISTINCT FROM o->>'created_at' OR n->>'created_by_id' IS DISTINCT FROM o->>'created_by_id' THEN RAISE EXCEPTION 'Sourcing offer ownership must be retained';END IF;
  END LOOP;
 END IF;
 RETURN NEW;END $$;
CREATE TRIGGER purchase_sourcing_scope BEFORE INSERT OR UPDATE ON purchase_sourcing_cases FOR EACH ROW EXECUTE FUNCTION protect_purchase_sourcing();
CREATE TABLE purchase_sourcing_events(id uuid PRIMARY KEY,subject_id uuid NOT NULL REFERENCES purchase_sourcing_cases(id),action varchar(30) NOT NULL CHECK(action IN('configure','rfq','dispatch','no_bid','quote','propose','decide','reopen','release')),reason varchar(2000) NOT NULL CHECK(length(btrim(reason))>0),before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,created_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id),FOREIGN KEY(id) REFERENCES business_events(id) DEFERRABLE INITIALLY DEFERRED);
CREATE FUNCTION protect_purchase_sourcing_history() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Sourcing records and history must be retained';END $$;
CREATE TRIGGER purchase_sourcing_no_delete BEFORE DELETE ON purchase_sourcing_cases FOR EACH ROW EXECUTE FUNCTION protect_purchase_sourcing_history();
CREATE TRIGGER purchase_sourcing_no_truncate BEFORE TRUNCATE ON purchase_sourcing_cases FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_sourcing_history();
CREATE TRIGGER purchase_sourcing_event_immutable BEFORE UPDATE OR DELETE ON purchase_sourcing_events FOR EACH ROW EXECUTE FUNCTION protect_purchase_sourcing_history();
CREATE TRIGGER purchase_sourcing_event_no_truncate BEFORE TRUNCATE ON purchase_sourcing_events FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_sourcing_history();
CREATE FUNCTION check_purchase_sourcing_event() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE c purchase_sourcing_cases;BEGIN
 SELECT * INTO c FROM purchase_sourcing_cases WHERE id=NEW.subject_id;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_sourcing' AND entity_type='sourcing_case' AND entity_id=NEW.subject_id AND entity_version=(NEW.after_snapshot->>'version')::integer AND action=NEW.action AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=c.project_id AND visibility='restricted') THEN RAISE EXCEPTION 'Sourcing history requires its immutable project event';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_sourcing_event_integrity AFTER INSERT ON purchase_sourcing_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_sourcing_event();
CREATE FUNCTION check_purchase_sourcing_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_sourcing_events WHERE subject_id=NEW.id AND after_snapshot=NEW.data AND created_by_id=NEW.updated_by_id AND created_at=NEW.updated_at AND (TG_OP='INSERT' OR before_snapshot=OLD.data)) THEN RAISE EXCEPTION 'Every sourcing write requires its exact immutable history';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_sourcing_write_integrity AFTER INSERT OR UPDATE ON purchase_sourcing_cases DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_sourcing_write();

CREATE TABLE purchase_sourcing_allocations(
 case_id uuid PRIMARY KEY REFERENCES purchase_sourcing_cases(id),project_id uuid NOT NULL REFERENCES projects(id),demand_id uuid NOT NULL REFERENCES project_material_requirements(id),
 financial_reference varchar(120) NOT NULL,currency varchar(3) NOT NULL,quantity numeric(19,4) NOT NULL CHECK(quantity>0),amount numeric(16,2) NOT NULL CHECK(amount>=0),
 approved_quantity numeric(19,4) NOT NULL CHECK(approved_quantity>0),approved_amount numeric(16,2) NOT NULL CHECK(approved_amount>=0),
 state varchar(20) NOT NULL CHECK(state IN('pending','approved','released','cancelled')),case_version integer NOT NULL CHECK(case_version>0),is_active boolean NOT NULL,
 created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id),
 CHECK(is_active=(state<>'cancelled')),CHECK(quantity<=approved_quantity),CHECK(amount<=approved_amount)
);
CREATE INDEX purchase_sourcing_allocation_balance ON purchase_sourcing_allocations(project_id,demand_id) WHERE is_active;
CREATE FUNCTION protect_purchase_sourcing_allocation() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE q numeric;a numeric;BEGIN
 PERFORM pg_advisory_xact_lock(hashtextextended(NEW.project_id::text||':'||NEW.demand_id::text,0));
 IF NOT EXISTS(SELECT 1 FROM purchase_sourcing_cases WHERE id=NEW.case_id AND project_id=NEW.project_id AND source_snapshot->>'demand_id'=NEW.demand_id::text) THEN RAISE EXCEPTION 'Allocation must belong to its exact sourcing demand';END IF;
 IF TG_OP='UPDATE' AND (OLD.state='released' OR NEW.case_id<>OLD.case_id OR NEW.project_id<>OLD.project_id OR NEW.demand_id<>OLD.demand_id OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.case_version<=OLD.case_version) THEN RAISE EXCEPTION 'Allocation identity, committed LOI and audit ownership must be retained';END IF;
 IF TG_OP='UPDATE' AND OLD.is_active AND NEW.is_active AND (NEW.financial_reference<>OLD.financial_reference OR NEW.currency<>OLD.currency OR NEW.quantity<>OLD.quantity OR NEW.amount<>OLD.amount OR NEW.approved_quantity<>OLD.approved_quantity OR NEW.approved_amount<>OLD.approved_amount) THEN RAISE EXCEPTION 'Submitted allocation evidence is immutable';END IF;
 IF NEW.is_active THEN
  SELECT coalesce(sum(quantity),0),coalesce(sum(amount) FILTER(WHERE financial_reference=NEW.financial_reference AND currency=NEW.currency),0) INTO q,a FROM purchase_sourcing_allocations WHERE project_id=NEW.project_id AND demand_id=NEW.demand_id AND is_active AND case_id<>NEW.case_id;
  IF q+NEW.quantity>NEW.approved_quantity OR a+NEW.amount>NEW.approved_amount THEN RAISE EXCEPTION 'Approved purchasing capacity already reserved';END IF;
 END IF;
 RETURN NEW;END $$;
CREATE TRIGGER purchase_sourcing_allocation_guard BEFORE INSERT OR UPDATE ON purchase_sourcing_allocations FOR EACH ROW EXECUTE FUNCTION protect_purchase_sourcing_allocation();
CREATE TRIGGER purchase_sourcing_allocation_no_delete BEFORE DELETE ON purchase_sourcing_allocations FOR EACH ROW EXECUTE FUNCTION protect_purchase_sourcing_history();
CREATE TRIGGER purchase_sourcing_allocation_no_truncate BEFORE TRUNCATE ON purchase_sourcing_allocations FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_sourcing_history();
CREATE FUNCTION check_purchase_sourcing_allocation() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE c purchase_sourcing_cases;BEGIN
 SELECT * INTO c FROM purchase_sourcing_cases WHERE id=NEW.case_id;
 IF c.version<>NEW.case_version OR c.updated_at<>NEW.updated_at OR c.updated_by_id IS DISTINCT FROM NEW.updated_by_id OR (NEW.is_active AND (c.state<>NEW.state OR c.source_snapshot->>'currency'<>NEW.currency OR (c.source_snapshot->>'quantity')::numeric<>NEW.quantity OR c.data->'decision_snapshot'->>'financial_reference' IS DISTINCT FROM NEW.financial_reference)) OR (NOT NEW.is_active AND c.state<>'draft') THEN RAISE EXCEPTION 'Allocation requires matching current sourcing state and audit';END IF;
 IF NOT EXISTS(SELECT 1 FROM purchase_sourcing_events WHERE subject_id=NEW.case_id AND (after_snapshot->>'version')::integer=NEW.case_version AND created_by_id=NEW.updated_by_id AND created_at=NEW.updated_at) THEN RAISE EXCEPTION 'Allocation requires exact sourcing history';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_sourcing_allocation_integrity AFTER INSERT OR UPDATE ON purchase_sourcing_allocations DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_sourcing_allocation();
CREATE FUNCTION check_purchase_sourcing_reservation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.state<>'draft' AND NOT EXISTS(SELECT 1 FROM purchase_sourcing_allocations WHERE case_id=NEW.id AND is_active AND state=NEW.state AND case_version=NEW.version) THEN RAISE EXCEPTION 'Submitted sourcing requires its owning allocation';END IF;
 IF NEW.state='draft' AND EXISTS(SELECT 1 FROM purchase_sourcing_allocations WHERE case_id=NEW.id AND is_active) THEN RAISE EXCEPTION 'Draft sourcing cannot retain a reservation';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_sourcing_reservation_integrity AFTER INSERT OR UPDATE ON purchase_sourcing_cases DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_sourcing_reservation();
COMMIT;
