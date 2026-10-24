BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM tender_provision_operations) OR EXISTS(SELECT 1 FROM tender_capture_operations) THEN RAISE EXCEPTION 'Cannot discard Tender capture or delivery evidence';END IF;END $$;
DROP TABLE tender_email_attempts,tender_email_jobs,tender_source_actions,tender_source_links,tender_capture_operations,tender_provision_operations;
ALTER TABLE tender_fms_intakes DROP COLUMN receipt_id,DROP COLUMN external_reference,DROP COLUMN source_url;
COMMIT;
