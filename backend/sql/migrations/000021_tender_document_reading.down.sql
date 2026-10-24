BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM document_readings) OR EXISTS(SELECT 1 FROM tender_fms_intakes) THEN RAISE EXCEPTION 'Cannot remove populated tender/source evidence'; END IF; END $$;
DROP TABLE tender_fms_events,document_reading_events,document_reading_results,document_readings,tender_fms_intakes;
DROP FUNCTION protect_reading_source();DROP FUNCTION protect_reading_evidence();COMMIT;
