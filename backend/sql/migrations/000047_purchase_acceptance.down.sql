BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM purchase_acceptances) THEN RAISE EXCEPTION 'Cannot rollback populated purchase acceptance';END IF;END $$;
DROP TABLE purchase_acceptance_events,purchase_acceptances;
DROP FUNCTION protect_purchase_acceptance(),protect_purchase_acceptance_history(),check_purchase_acceptance_write(),check_purchase_acceptance_event();
COMMIT;
