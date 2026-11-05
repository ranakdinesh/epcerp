BEGIN;
-- Existing Go projects already exposed an assigned Purchase FMS workspace. Preserve
-- that native behaviour; new registrations require an explicit captured choice.
ALTER TABLE projects ADD COLUMN purchase_enabled boolean NOT NULL DEFAULT true;
ALTER TABLE projects ALTER COLUMN purchase_enabled SET DEFAULT false;
COMMIT;
