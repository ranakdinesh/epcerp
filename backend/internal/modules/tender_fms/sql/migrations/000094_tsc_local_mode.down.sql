BEGIN;
-- Refuse downgrade while retained local draft evidence exists; never delete it.
ALTER TABLE tender.tsc_jobs DROP CONSTRAINT tsc_jobs_mode_check;
ALTER TABLE tender.tsc_jobs ADD CONSTRAINT tsc_jobs_mode_check CHECK(mode IN ('manual','ollama','gemini','openai'));
COMMIT;
