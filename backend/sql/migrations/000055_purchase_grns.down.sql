BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM purchase_grns) THEN RAISE EXCEPTION 'Cannot rollback populated GRNs';END IF;END $$;
DROP TABLE purchase_grns_events,purchase_grns;
DROP FUNCTION protect_purchase_grns(),protect_purchase_grns_history(),check_purchase_grns_write(),check_purchase_grns_event();
COMMIT;
