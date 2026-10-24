DO $$ BEGIN IF EXISTS(SELECT 1 FROM project_imports) THEN RAISE EXCEPTION 'Cannot discard imported project provenance'; END IF; END $$;
DROP TABLE project_imports;DROP FUNCTION reject_project_import_change();
