BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM purchase_engineer_confirmations) THEN RAISE EXCEPTION 'Cannot rollback populated engineer confirmations';END IF;END $$;
DROP TABLE purchase_engineer_confirmations_events,purchase_engineer_confirmations;
DROP FUNCTION protect_purchase_engineer_confirmations(),protect_purchase_engineer_confirmations_history(),check_purchase_engineer_confirmations_write(),check_purchase_engineer_confirmations_event();
COMMIT;
