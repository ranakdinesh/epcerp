DO $$ BEGIN IF EXISTS(SELECT 1 FROM project_metadata_changes) THEN RAISE EXCEPTION 'Cannot roll back populated project metadata history'; END IF; END $$;
DROP TABLE project_metadata_changes;
DROP FUNCTION reject_project_metadata_change();
