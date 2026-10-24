BEGIN;
CREATE TABLE finance_budget_decisions (
 id uuid PRIMARY KEY,project_id uuid NOT NULL REFERENCES projects(id),source_version integer NOT NULL CHECK(source_version>0),source_revision integer NOT NULL CHECK(source_revision>0),
 amount numeric(16,2) NOT NULL CHECK(amount>0),currency char(3) NOT NULL CHECK(currency~'^[A-Z]{3}$'),source_fingerprint char(64) NOT NULL CHECK(source_fingerprint~'^[0-9a-f]{64}$'),reason varchar(2000) NOT NULL CHECK(length(btrim(reason))>0),is_active boolean NOT NULL CHECK(is_active),
 created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id),UNIQUE(project_id,source_revision),
 FOREIGN KEY(id) REFERENCES business_events(id) DEFERRABLE INITIALLY DEFERRED,
 CHECK(updated_at=created_at),CHECK(updated_by_id IS NOT DISTINCT FROM created_by_id)
);
CREATE FUNCTION protect_finance_budget_decisions() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Finance decisions are immutable';END $$;
CREATE TRIGGER finance_budget_decision_immutable BEFORE UPDATE OR DELETE ON finance_budget_decisions FOR EACH ROW EXECUTE FUNCTION protect_finance_budget_decisions();
CREATE TRIGGER finance_budget_decision_no_truncate BEFORE TRUNCATE ON finance_budget_decisions FOR EACH STATEMENT EXECUTE FUNCTION protect_finance_budget_decisions();
ALTER TABLE project_budgets DROP CONSTRAINT project_budgets_state_check;
ALTER TABLE project_budgets ADD CONSTRAINT project_budgets_state_check CHECK(state IN('draft','approved'));
ALTER TABLE project_budgets ADD COLUMN approval_decision_id uuid REFERENCES finance_budget_decisions(id),ADD COLUMN approved_snapshot jsonb NOT NULL DEFAULT 'null'::jsonb;
ALTER TABLE project_budgets ADD CONSTRAINT project_budget_approval_snapshot CHECK((approval_decision_id IS NULL)=(approved_snapshot='null'::jsonb) AND (state<>'approved' OR approval_decision_id IS NOT NULL));
ALTER TABLE project_budget_events DROP CONSTRAINT project_budget_events_action_check;
ALTER TABLE project_budget_events ADD CONSTRAINT project_budget_events_action_check CHECK(action IN('save','price_items','approve','revise'));
CREATE FUNCTION check_project_budget_finance_baseline() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE d finance_budget_decisions;BEGIN
 IF NEW.approval_decision_id IS NOT NULL THEN
  SELECT * INTO d FROM finance_budget_decisions WHERE id=NEW.approval_decision_id;
  IF NOT FOUND OR d.project_id IS DISTINCT FROM NEW.project_id OR d.source_revision IS DISTINCT FROM (NEW.approved_snapshot->>'revision')::integer OR d.source_revision>NEW.revision OR d.currency IS DISTINCT FROM (NEW.approved_snapshot->'data'->>'currency') OR d.amount IS DISTINCT FROM (NEW.approved_snapshot->'data'->>'total')::numeric OR d.created_by_id::text IS DISTINCT FROM (NEW.approved_snapshot->>'approved_by_id') OR d.id::text IS DISTINCT FROM (NEW.approved_snapshot->>'decision_id') THEN RAISE EXCEPTION 'Approved baseline requires its matching Finance decision';END IF;
 END IF;
 IF TG_OP='UPDATE' THEN
  IF NEW.approval_decision_id IS NOT DISTINCT FROM OLD.approval_decision_id AND NEW.approved_snapshot IS DISTINCT FROM OLD.approved_snapshot THEN RAISE EXCEPTION 'Approved budget baseline is immutable';END IF;
  IF OLD.state='approved' AND NEW.state='approved' THEN RAISE EXCEPTION 'Start a budget revision before changing an approved baseline';END IF;
  IF OLD.state='approved' AND (NEW.state<>'draft' OR NEW.revision<>OLD.revision+1 OR NEW.approval_decision_id IS DISTINCT FROM OLD.approval_decision_id OR NEW.approved_snapshot IS DISTINCT FROM OLD.approved_snapshot) THEN RAISE EXCEPTION 'A revision must retain the prior approved baseline';END IF;
 END IF;
 IF NEW.state='approved' AND (d.source_version<>NEW.version-1 OR d.source_revision<>NEW.revision OR d.amount<>NEW.total OR d.currency<>NEW.currency OR NEW.updated_by_id IS DISTINCT FROM d.created_by_id) THEN RAISE EXCEPTION 'Budget approval requires its current source version and actor';END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER project_budget_finance_baseline BEFORE INSERT OR UPDATE ON project_budgets FOR EACH ROW EXECUTE FUNCTION check_project_budget_finance_baseline();
CREATE FUNCTION check_finance_budget_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='finance_approvals' AND entity_type='financial_decision' AND entity_id=NEW.id AND entity_version=1 AND action='approved' AND actor_id=NEW.created_by_id AND scope_kind='project' AND scope_id=NEW.project_id AND visibility='restricted') THEN RAISE EXCEPTION 'Finance decision requires its immutable project event';END IF;
 RETURN NEW;
END $$;
CREATE CONSTRAINT TRIGGER finance_budget_event AFTER INSERT ON finance_budget_decisions DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_finance_budget_event();
COMMIT;
