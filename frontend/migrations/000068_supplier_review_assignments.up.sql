BEGIN;
CREATE TABLE supplier_review_assignments (
 id uuid NOT NULL UNIQUE, supplier_id uuid PRIMARY KEY REFERENCES suppliers(id), user_id uuid NOT NULL REFERENCES users(id),
 version integer NOT NULL CHECK(version>0), is_active boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT now(), created_by_id uuid REFERENCES users(id) ON DELETE SET NULL,
 updated_at timestamptz NOT NULL DEFAULT now(), updated_by_id uuid REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX supplier_review_assignee ON supplier_review_assignments(user_id,supplier_id) WHERE is_active;
CREATE TABLE supplier_review_assignment_changes (
 id uuid PRIMARY KEY, supplier_id uuid NOT NULL REFERENCES suppliers(id), version integer NOT NULL CHECK(version>0),
 reason varchar(2000) NOT NULL CHECK(length(trim(reason))>0), snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), created_by_id uuid REFERENCES users(id) ON DELETE SET NULL, UNIQUE(supplier_id,version)
);
CREATE TRIGGER supplier_review_assignment_history_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON supplier_review_assignment_changes FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
ALTER TABLE communication_notices ALTER COLUMN project_id DROP NOT NULL;
ALTER TABLE communication_notices DROP CONSTRAINT communication_notices_notice_kind_check;
ALTER TABLE communication_notices ADD CONSTRAINT communication_notices_notice_kind_check CHECK(notice_kind IN('director','quotation','supplier_review'));
ALTER TABLE communication_notices DROP CONSTRAINT communication_action;
ALTER TABLE communication_notices ADD CONSTRAINT communication_action CHECK(decision_action IN('approved','rejected','returned','cancelled','submit','accounts_decision','assigned'));
CREATE OR REPLACE FUNCTION protect_communication_notice() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF TG_OP='INSERT' THEN
  IF NEW.version<>1 OR NEW.read_at IS NOT NULL OR NEW.acknowledged_at IS NOT NULL OR NEW.created_at<>NEW.updated_at OR NEW.created_by_id IS DISTINCT FROM NEW.updated_by_id THEN RAISE EXCEPTION 'New notice must be unread with source attribution';END IF;
  IF NEW.notice_kind='director' THEN
  IF NEW.project_id IS NULL THEN RAISE EXCEPTION 'Director notice requires project'; END IF;
  IF NOT EXISTS(SELECT 1 FROM business_events e WHERE e.id=NEW.source_event_id AND e.source_module IN('finance_approvals','finance_accounts_acceptance') AND e.entity_type='financial_decision' AND e.entity_id=NEW.decision_id AND e.entity_version=NEW.source_version AND e.action=NEW.decision_action AND e.actor_id=NEW.created_by_id AND e.actor_kind='user' AND e.scope_kind='project' AND e.scope_id=NEW.project_id AND e.visibility='restricted' AND e.occurred_at=NEW.created_at) THEN RAISE EXCEPTION 'Notice source must be its recorded financial decision';END IF;
  IF NOT EXISTS(SELECT 1 FROM organisation_director_designation d JOIN users u ON u.id=d.user_id WHERE d.id=NEW.director_designation_id AND d.version=NEW.director_version AND d.user_id=NEW.recipient_id AND d.is_active AND u.is_active) THEN RAISE EXCEPTION 'Notice requires the effective Director';END IF;
  ELSIF NEW.notice_kind='supplier_review' THEN
   IF NEW.project_id IS NOT NULL OR NEW.director_designation_id IS NOT NULL OR NEW.director_version<>0 OR NEW.title='' OR NOT EXISTS(
    SELECT 1 FROM business_events e JOIN supplier_review_assignments a ON a.id=e.entity_id
    WHERE e.id=NEW.source_event_id AND e.source_module='supplier_review_assignments' AND e.entity_type='supplier_review_assignment'
    AND e.entity_id=NEW.decision_id AND e.entity_version=NEW.source_version AND e.action='assigned' AND NEW.decision_action='assigned'
    AND e.actor_id=NEW.created_by_id AND e.actor_kind='user' AND e.scope_kind='platform' AND e.scope_id IS NULL
    AND e.visibility='restricted' AND e.occurred_at=NEW.created_at AND a.version=NEW.source_version AND a.user_id=NEW.recipient_id AND a.is_active
    AND NEW.link='/dashboard?screen=suppliers&supplier_id='||a.supplier_id::text
   ) THEN RAISE EXCEPTION 'Supplier review notice requires current recorded assignment';END IF;
  ELSE
   IF NEW.project_id IS NULL THEN RAISE EXCEPTION 'Quotation notice requires project'; END IF;
   IF NEW.director_designation_id IS NOT NULL OR NEW.director_version<>0 OR NEW.title='' OR NEW.link !~ '^/dashboard\?' OR NOT EXISTS(SELECT 1 FROM business_events e WHERE e.id=NEW.source_event_id AND e.source_module='quotation_approvals' AND e.entity_type='quotation_approval_set' AND e.entity_id=NEW.decision_id AND e.entity_version=NEW.source_version AND e.action=NEW.decision_action AND e.action IN('submit','accounts_decision') AND e.actor_id=NEW.created_by_id AND e.scope_kind='project' AND e.scope_id=NEW.project_id AND e.visibility='restricted' AND e.occurred_at=NEW.created_at) THEN RAISE EXCEPTION 'Addressed quotation notice requires its recorded source';END IF;
  END IF;
 ELSE
  IF ROW(NEW.notice_kind,NEW.title,NEW.link,NEW.id,NEW.source_event_id,NEW.recipient_id,NEW.director_designation_id,NEW.director_version,NEW.decision_id,NEW.project_id,NEW.source_version,NEW.decision_action,NEW.is_active,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.notice_kind,OLD.title,OLD.link,OLD.id,OLD.source_event_id,OLD.recipient_id,OLD.director_designation_id,OLD.director_version,OLD.decision_id,OLD.project_id,OLD.source_version,OLD.decision_action,OLD.is_active,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Notice identity and addressed source are immutable';END IF;
  IF NEW.version<>OLD.version+1 OR NEW.updated_by_id IS DISTINCT FROM NEW.recipient_id OR NEW.updated_at<OLD.updated_at OR (OLD.read_at IS NOT NULL AND NEW.read_at IS DISTINCT FROM OLD.read_at) OR (OLD.acknowledged_at IS NOT NULL AND NEW.acknowledged_at IS DISTINCT FROM OLD.acknowledged_at) OR ROW(NEW.read_at,NEW.acknowledged_at) IS NOT DISTINCT FROM ROW(OLD.read_at,OLD.acknowledged_at) THEN RAISE EXCEPTION 'Notice receipts must advance monotonically for the recipient';END IF;
 END IF;RETURN NEW;END $$;

COMMIT;
