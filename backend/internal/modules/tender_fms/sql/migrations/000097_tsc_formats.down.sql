BEGIN;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM tender.tsc_format_events) OR EXISTS(SELECT 1 FROM tender.tsc_jobs WHERE format_id IS NOT NULL)
 OR EXISTS(SELECT 1 FROM tender.tsc_formats WHERE id<>'00000000-0000-4000-8000-000000000097' OR version<>1)
 THEN RAISE EXCEPTION 'TSC format history exists; use a forward migration'; END IF;
END $$;
DROP TRIGGER tsc_format_snapshot_immutable ON tender.tsc_jobs;
DROP FUNCTION tender.protect_tsc_format_snapshot();
ALTER TABLE tender.tsc_jobs DROP CONSTRAINT tsc_job_format_pair, DROP COLUMN format_id, DROP COLUMN format_version;
DROP TABLE tender.tsc_format_events,tender.tsc_format_questions,tender.tsc_formats;
DROP FUNCTION tender.tsc_question_key(text);
COMMIT;
