BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM project_responsibility_profiles) OR EXISTS(SELECT 1 FROM project_responsibility_events) THEN RAISE EXCEPTION 'Refuse to remove populated project responsibility evidence';END IF;END $$;
DROP TABLE project_responsibility_events,project_responsibility_profiles;
DROP FUNCTION protect_project_responsibility_scope(),protect_project_responsibility_evidence();
COMMIT;
