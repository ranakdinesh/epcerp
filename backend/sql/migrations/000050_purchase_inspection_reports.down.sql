BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM purchase_inspection_reports) THEN RAISE EXCEPTION 'Cannot rollback populated inspection reports';END IF;END $$;
DROP TABLE purchase_inspection_report_events,purchase_inspection_reports;
DROP FUNCTION protect_purchase_inspection_report(),protect_purchase_inspection_report_history(),check_purchase_inspection_report_write(),check_purchase_inspection_report_event();
COMMIT;
