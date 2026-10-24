BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM purchase_inspections) THEN RAISE EXCEPTION 'Cannot rollback populated purchase inspections';END IF;END $$;
DROP TABLE purchase_inspection_events,purchase_inspections;
DROP FUNCTION protect_purchase_inspection(),protect_purchase_inspection_history(),check_purchase_inspection_write(),check_purchase_inspection_event();
COMMIT;
