BEGIN;
CREATE TABLE quotation_approval_sets(
 bo_id uuid PRIMARY KEY REFERENCES purchase_bo_lists(id),project_id uuid NOT NULL REFERENCES projects(id),version integer NOT NULL CHECK(version>0),attempts jsonb NOT NULL CHECK(jsonb_typeof(attempts)='array'),is_active boolean NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id),UNIQUE(bo_id,project_id)
);
CREATE FUNCTION protect_quotation_approval_scope() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE a jsonb;n jsonb;l jsonb;nl jsonb;BEGIN
 IF TG_OP='UPDATE' AND (NEW.bo_id IS DISTINCT FROM OLD.bo_id OR NEW.project_id IS DISTINCT FROM OLD.project_id OR NEW.created_at IS DISTINCT FROM OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1) THEN RAISE EXCEPTION 'Quotation approval ownership/version is immutable';END IF;
 IF TG_OP='UPDATE' THEN
  FOR a IN SELECT value FROM jsonb_array_elements(OLD.attempts) LOOP
   SELECT value INTO n FROM jsonb_array_elements(NEW.attempts) WHERE value->>'id'=a->>'id';
   IF NOT FOUND THEN RAISE EXCEPTION 'Quotation submissions must be retained';END IF;
   IF a->>'state'<>'pending_accounts' AND n IS DISTINCT FROM a THEN RAISE EXCEPTION 'Decided quotations are immutable';END IF;
   IF n-ARRAY['state','lines','reviewed_by_id','reviewed_at','decision_id'] IS DISTINCT FROM a-ARRAY['state','lines','reviewed_by_id','reviewed_at','decision_id'] THEN RAISE EXCEPTION 'Submitted quotation source is immutable';END IF;
   IF jsonb_array_length(n->'lines')<>jsonb_array_length(a->'lines') THEN RAISE EXCEPTION 'Every submitted item must be retained';END IF;
   FOR l IN SELECT value FROM jsonb_array_elements(a->'lines') LOOP
    SELECT value INTO nl FROM jsonb_array_elements(n->'lines') WHERE value->>'line_id'=l->>'line_id';
    IF NOT FOUND OR nl-ARRAY['decision','comment'] IS DISTINCT FROM l-ARRAY['decision','comment'] THEN RAISE EXCEPTION 'Submitted item source is immutable';END IF;
   END LOOP;
  END LOOP;
 END IF;
 IF NOT EXISTS(SELECT 1 FROM purchase_bo_lists WHERE id=NEW.bo_id AND project_id=NEW.project_id) THEN RAISE EXCEPTION 'Quotation approval BO belongs to another project';END IF;
 RETURN NEW;END $$;
CREATE TRIGGER quotation_approval_scope BEFORE INSERT OR UPDATE ON quotation_approval_sets FOR EACH ROW EXECUTE FUNCTION protect_quotation_approval_scope();
CREATE TABLE quotation_approval_events(id uuid PRIMARY KEY,bo_id uuid NOT NULL REFERENCES quotation_approval_sets(bo_id),action varchar(40) NOT NULL CHECK(action IN('submit','accounts_decision')),reason varchar(2000) NOT NULL CHECK(length(btrim(reason))>0),before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,created_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id));
CREATE FUNCTION protect_quotation_approval_history() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Quotation approval records/history must be retained';END $$;
CREATE TRIGGER quotation_approval_no_delete BEFORE DELETE ON quotation_approval_sets FOR EACH ROW EXECUTE FUNCTION protect_quotation_approval_history();
CREATE TRIGGER quotation_approval_no_truncate BEFORE TRUNCATE ON quotation_approval_sets FOR EACH STATEMENT EXECUTE FUNCTION protect_quotation_approval_history();
CREATE TRIGGER quotation_approval_history_immutable BEFORE UPDATE OR DELETE ON quotation_approval_events FOR EACH ROW EXECUTE FUNCTION protect_quotation_approval_history();
CREATE TRIGGER quotation_approval_history_no_truncate BEFORE TRUNCATE ON quotation_approval_events FOR EACH STATEMENT EXECUTE FUNCTION protect_quotation_approval_history();
CREATE TABLE finance_quotation_decisions(
 id uuid PRIMARY KEY,project_id uuid NOT NULL,bo_id uuid NOT NULL,attempt_id uuid NOT NULL UNIQUE,source_version integer NOT NULL CHECK(source_version>0),submitted_by_id uuid NOT NULL REFERENCES users(id),outcome varchar(20) NOT NULL CHECK(outcome IN('approved','rejected')),amount numeric(16,2) NOT NULL CHECK(amount>=0),currency char(3) NOT NULL CHECK(currency~'^[A-Z]{3}$'),source_fingerprint char(64) NOT NULL CHECK(source_fingerprint~'^[0-9a-f]{64}$'),reason varchar(2000) NOT NULL CHECK(length(btrim(reason))>0),is_active boolean NOT NULL CHECK(is_active),
 created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id),CHECK(created_by_id IS DISTINCT FROM submitted_by_id),CHECK(updated_at=created_at),CHECK(updated_by_id IS NOT DISTINCT FROM created_by_id),
 FOREIGN KEY(bo_id,project_id) REFERENCES quotation_approval_sets(bo_id,project_id),FOREIGN KEY(id) REFERENCES business_events(id) DEFERRABLE INITIALLY DEFERRED
);
CREATE TRIGGER finance_quotation_immutable BEFORE UPDATE OR DELETE ON finance_quotation_decisions FOR EACH ROW EXECUTE FUNCTION protect_finance_budget_decisions();
CREATE TRIGGER finance_quotation_no_truncate BEFORE TRUNCATE ON finance_quotation_decisions FOR EACH STATEMENT EXECUTE FUNCTION protect_finance_budget_decisions();
CREATE FUNCTION check_finance_quotation_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='finance_approvals' AND entity_type='financial_decision' AND entity_id=NEW.id AND entity_version=1 AND action=NEW.outcome AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=NEW.project_id AND visibility='restricted') THEN RAISE EXCEPTION 'Quotation Finance decision requires its immutable project event';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER finance_quotation_event AFTER INSERT ON finance_quotation_decisions DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_finance_quotation_event();
CREATE FUNCTION check_quotation_approval_decisions() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE a jsonb;d finance_quotation_decisions;BEGIN
 FOR a IN SELECT value FROM jsonb_array_elements(NEW.attempts) LOOP
  IF a->>'state' IN('approved','rejected') THEN
   IF EXISTS(SELECT 1 FROM jsonb_array_elements(a->'lines') l WHERE l->>'decision' NOT IN('accept','reject') OR l->>'decision' IS NULL OR coalesce(length(btrim(l->>'comment')),0)=0) OR (a->>'state'='rejected') IS DISTINCT FROM EXISTS(SELECT 1 FROM jsonb_array_elements(a->'lines') l WHERE l->>'decision'='reject') THEN RAISE EXCEPTION 'Every item decision must match the Finance outcome';END IF;
   SELECT * INTO d FROM finance_quotation_decisions WHERE id=(a->>'decision_id')::uuid AND attempt_id=(a->>'id')::uuid AND bo_id=NEW.bo_id AND project_id=NEW.project_id;
   IF NOT FOUND OR d.outcome IS DISTINCT FROM a->>'state' OR d.currency IS DISTINCT FROM a->>'currency' OR d.amount IS DISTINCT FROM (a->>'total')::numeric OR d.submitted_by_id::text IS DISTINCT FROM a->>'submitted_by_id' OR d.created_by_id::text IS DISTINCT FROM a->>'reviewed_by_id' OR d.source_version>=NEW.version OR a->>'reviewed_at' IS NULL THEN RAISE EXCEPTION 'Quotation outcome requires its matching Finance decision';END IF;
  ELSIF a->>'state' IS DISTINCT FROM 'pending_accounts' OR a->>'decision_id' IS NOT NULL OR a->>'reviewed_by_id' IS NOT NULL OR a->>'reviewed_at' IS NOT NULL THEN RAISE EXCEPTION 'Quotation request state is invalid';END IF;
 END LOOP;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER quotation_approval_finance AFTER INSERT OR UPDATE ON quotation_approval_sets DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_quotation_approval_decisions();
COMMIT;
