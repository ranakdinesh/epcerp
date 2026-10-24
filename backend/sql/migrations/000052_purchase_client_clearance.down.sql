BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM purchase_client_clearances) THEN RAISE EXCEPTION 'Cannot rollback populated client clearances';END IF;END $$;
DROP TABLE purchase_client_clearance_events,purchase_client_clearances;
DROP FUNCTION protect_purchase_client_clearance(),protect_purchase_client_clearance_history(),check_purchase_client_clearance_write(),check_purchase_client_clearance_event();
COMMIT;
