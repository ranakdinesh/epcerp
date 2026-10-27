BEGIN;
DROP TABLE tender.tsc_events;
DROP TABLE tender.tsc_jobs;
DROP FUNCTION tender.protect_tsc_inputs();
COMMIT;
