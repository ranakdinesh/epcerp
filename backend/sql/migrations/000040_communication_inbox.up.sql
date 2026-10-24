BEGIN;
CREATE TABLE communication_notices(
 id uuid PRIMARY KEY,source_event_id uuid NOT NULL UNIQUE REFERENCES business_events(id),recipient_id uuid NOT NULL REFERENCES users(id),director_designation_id uuid NOT NULL REFERENCES organisation_director_designation(id),director_version integer NOT NULL CHECK(director_version>0),
 decision_id uuid NOT NULL,project_id uuid NOT NULL REFERENCES projects(id),source_version bigint NOT NULL CHECK(source_version>0),decision_action varchar(30) NOT NULL CHECK(decision_action IN('approved','rejected','returned','cancelled')),
 version integer NOT NULL DEFAULT 1 CHECK(version>0),is_active boolean NOT NULL DEFAULT true CHECK(is_active),read_at timestamptz,acknowledged_at timestamptz,
 created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id),
 CHECK(acknowledged_at IS NULL OR read_at IS NOT NULL),CHECK(updated_at>=created_at),CHECK(read_at IS NULL OR read_at>=created_at),CHECK(acknowledged_at IS NULL OR acknowledged_at>=read_at)
);
CREATE INDEX communication_recipient_list ON communication_notices(recipient_id,created_at DESC,id DESC);
CREATE TABLE communication_notice_events(
 id uuid PRIMARY KEY,notice_id uuid NOT NULL REFERENCES communication_notices(id),action varchar(30) NOT NULL CHECK(action IN('delivered','read','acknowledge')),notice_version integer NOT NULL CHECK(notice_version>0),before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id),UNIQUE(notice_id,notice_version)
);
CREATE FUNCTION protect_communication_notice() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF TG_OP='INSERT' THEN
  IF NEW.version<>1 OR NEW.read_at IS NOT NULL OR NEW.acknowledged_at IS NOT NULL OR NEW.created_at<>NEW.updated_at OR NEW.created_by_id IS DISTINCT FROM NEW.updated_by_id THEN RAISE EXCEPTION 'New notice must be unread with source attribution';END IF;
  IF NOT EXISTS(SELECT 1 FROM business_events e WHERE e.id=NEW.source_event_id AND e.source_module='finance_approvals' AND e.entity_type='financial_decision' AND e.entity_id=NEW.decision_id AND e.entity_version=NEW.source_version AND e.action=NEW.decision_action AND e.actor_id=NEW.created_by_id AND e.actor_kind='user' AND e.scope_kind='project' AND e.scope_id=NEW.project_id AND e.visibility='restricted' AND e.occurred_at=NEW.created_at) THEN RAISE EXCEPTION 'Notice source must be its recorded financial decision';END IF;
  IF NOT EXISTS(SELECT 1 FROM organisation_director_designation d JOIN users u ON u.id=d.user_id WHERE d.id=NEW.director_designation_id AND d.version=NEW.director_version AND d.user_id=NEW.recipient_id AND d.is_active AND u.is_active) THEN RAISE EXCEPTION 'Notice requires the effective Director';END IF;
 ELSE
  IF ROW(NEW.id,NEW.source_event_id,NEW.recipient_id,NEW.director_designation_id,NEW.director_version,NEW.decision_id,NEW.project_id,NEW.source_version,NEW.decision_action,NEW.is_active,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.id,OLD.source_event_id,OLD.recipient_id,OLD.director_designation_id,OLD.director_version,OLD.decision_id,OLD.project_id,OLD.source_version,OLD.decision_action,OLD.is_active,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Notice identity and addressed source are immutable';END IF;
  IF NEW.version<>OLD.version+1 OR NEW.updated_by_id IS DISTINCT FROM NEW.recipient_id OR NEW.updated_at<OLD.updated_at OR (OLD.read_at IS NOT NULL AND NEW.read_at IS DISTINCT FROM OLD.read_at) OR (OLD.acknowledged_at IS NOT NULL AND NEW.acknowledged_at IS DISTINCT FROM OLD.acknowledged_at) OR ROW(NEW.read_at,NEW.acknowledged_at) IS NOT DISTINCT FROM ROW(OLD.read_at,OLD.acknowledged_at) THEN RAISE EXCEPTION 'Notice receipts must advance monotonically for the recipient';END IF;
 END IF;RETURN NEW;END $$;
CREATE TRIGGER communication_notice_identity BEFORE INSERT OR UPDATE ON communication_notices FOR EACH ROW EXECUTE FUNCTION protect_communication_notice();
CREATE TRIGGER communication_notices_no_delete BEFORE DELETE OR TRUNCATE ON communication_notices FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
CREATE TRIGGER communication_notice_events_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON communication_notice_events FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
COMMIT;
