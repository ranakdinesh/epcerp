BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM finance_accounts_acceptance) THEN RAISE EXCEPTION 'Cannot rollback populated Accounts acceptance';END IF;END $$;
DROP TABLE finance_accounts_acceptance_events,finance_accounts_acceptance;
DROP FUNCTION protect_finance_accounts_acceptance(),protect_finance_accounts_acceptance_history(),check_finance_accounts_acceptance_write(),check_finance_accounts_acceptance_event();
CREATE OR REPLACE FUNCTION protect_communication_notice() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF TG_OP='INSERT' THEN
  IF NEW.version<>1 OR NEW.read_at IS NOT NULL OR NEW.acknowledged_at IS NOT NULL OR NEW.created_at<>NEW.updated_at OR NEW.created_by_id IS DISTINCT FROM NEW.updated_by_id THEN RAISE EXCEPTION 'New notice must be unread with source attribution';END IF;
  IF NOT EXISTS(SELECT 1 FROM business_events e WHERE e.id=NEW.source_event_id AND e.source_module='finance_approvals' AND e.entity_type='financial_decision' AND e.entity_id=NEW.decision_id AND e.entity_version=NEW.source_version AND e.action=NEW.decision_action AND e.actor_id=NEW.created_by_id AND e.actor_kind='user' AND e.scope_kind='project' AND e.scope_id=NEW.project_id AND e.visibility='restricted' AND e.occurred_at=NEW.created_at) THEN RAISE EXCEPTION 'Notice source must be its recorded financial decision';END IF;
  IF NOT EXISTS(SELECT 1 FROM organisation_director_designation d JOIN users u ON u.id=d.user_id WHERE d.id=NEW.director_designation_id AND d.version=NEW.director_version AND d.user_id=NEW.recipient_id AND d.is_active AND u.is_active) THEN RAISE EXCEPTION 'Notice requires the effective Director';END IF;
 ELSE
  IF ROW(NEW.id,NEW.source_event_id,NEW.recipient_id,NEW.director_designation_id,NEW.director_version,NEW.decision_id,NEW.project_id,NEW.source_version,NEW.decision_action,NEW.is_active,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.id,OLD.source_event_id,OLD.recipient_id,OLD.director_designation_id,OLD.director_version,OLD.decision_id,OLD.project_id,OLD.source_version,OLD.decision_action,OLD.is_active,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Notice identity and addressed source are immutable';END IF;
  IF NEW.version<>OLD.version+1 OR NEW.updated_by_id IS DISTINCT FROM NEW.recipient_id OR NEW.updated_at<OLD.updated_at OR (OLD.read_at IS NOT NULL AND NEW.read_at IS DISTINCT FROM OLD.read_at) OR (OLD.acknowledged_at IS NOT NULL AND NEW.acknowledged_at IS DISTINCT FROM OLD.acknowledged_at) OR ROW(NEW.read_at,NEW.acknowledged_at) IS NOT DISTINCT FROM ROW(OLD.read_at,OLD.acknowledged_at) THEN RAISE EXCEPTION 'Notice receipts must advance monotonically for the recipient';END IF;
 END IF;RETURN NEW;END $$;
COMMIT;
