BEGIN;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM tender.tsc_cloud_calls) OR EXISTS(SELECT 1 FROM tender.tsc_jobs WHERE mode IN ('gemini','openai')) THEN
 RAISE EXCEPTION 'Retained cloud evidence prevents downgrade'; END IF;
END $$;
DROP TABLE tender.tsc_cloud_calls;
DROP FUNCTION tender.protect_cloud_reservation();
ALTER TABLE tender.tsc_jobs DROP CONSTRAINT tsc_jobs_mode_check;
ALTER TABLE tender.tsc_jobs ADD CONSTRAINT tsc_jobs_mode_check CHECK(mode IN ('manual','ollama'));
COMMIT;
