BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM purchase_dispatches) THEN RAISE EXCEPTION 'Cannot rollback populated dispatches';END IF;END $$;
DROP TABLE purchase_dispatch_events,purchase_dispatches;
DROP FUNCTION protect_purchase_dispatch(),protect_purchase_dispatch_history(),check_purchase_dispatch_write(),check_purchase_dispatch_event();
COMMIT;
