BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM purchase_quality_cases) THEN RAISE EXCEPTION 'Cannot rollback populated quality cases';END IF;END $$;
DROP TABLE purchase_quality_events,purchase_quality_certificates,purchase_quality_cases;
DROP FUNCTION protect_purchase_quality_case(),protect_purchase_quality_certificate(),protect_purchase_quality_history(),check_purchase_quality_children(),check_purchase_quality_write(),check_purchase_quality_event();
COMMIT;
