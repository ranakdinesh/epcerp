BEGIN;
ALTER TABLE tender.tsc_jobs DROP CONSTRAINT tsc_jobs_mode_check;
ALTER TABLE tender.tsc_jobs ADD CONSTRAINT tsc_jobs_mode_check CHECK(mode IN ('manual','local','ollama','gemini','openai'));
COMMIT;
