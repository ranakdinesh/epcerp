BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM purchase_site_receipts) THEN RAISE EXCEPTION 'Cannot rollback populated site receipts';END IF;END $$;
DROP TABLE purchase_site_receipts_events,purchase_site_receipts;
DROP FUNCTION protect_purchase_site_receipts(),protect_purchase_site_receipts_history(),check_purchase_site_receipts_write(),check_purchase_site_receipts_event();
COMMIT;
