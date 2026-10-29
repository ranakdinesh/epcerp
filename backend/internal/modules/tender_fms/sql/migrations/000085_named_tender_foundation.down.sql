BEGIN;
-- Evidence-bearing named records require explicit export/reconciliation; never silently discard them.
DO $$ BEGIN IF EXISTS(SELECT 1 FROM tender_fms_participant_grants) OR EXISTS(SELECT 1 FROM tender_fms_intakes WHERE record_kind='named') OR EXISTS(SELECT 1 FROM tender_fms_record_events) OR EXISTS(SELECT 1 FROM tender_fms_manager_assignments)
 THEN RAISE EXCEPTION 'Cannot discard named Tender records or assignment evidence'; END IF; END $$;
ALTER TABLE tender_fms_intakes DROP CONSTRAINT tender_fms_current_manager;
DROP TABLE tender_fms_participant_grants;
DROP TABLE tender_fms_record_events;
DROP TABLE tender_fms_manager_assignments;
DROP INDEX tender_fms_sales_lookup;
DROP INDEX tender_fms_legacy_fingerprint;
ALTER TABLE tender_fms_intakes DROP CONSTRAINT tender_fms_identity_kind,
 DROP COLUMN manager_assignment_id, DROP COLUMN assignment_version, DROP COLUMN content_revision,
 DROP COLUMN business_version, DROP COLUMN sales_creator_id, DROP COLUMN legacy_title, DROP COLUMN record_kind,
 ALTER COLUMN initial_sha256 SET NOT NULL,
 ADD CONSTRAINT tender_fms_intakes_owner_id_initial_sha256_key UNIQUE(owner_id,initial_sha256);
COMMIT;
