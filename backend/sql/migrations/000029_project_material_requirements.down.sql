BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM project_material_requirements) THEN RAISE EXCEPTION 'Populated project material evidence cannot be removed'; END IF; END $$;
DROP TABLE project_material_events,project_material_requirements;
DROP FUNCTION protect_project_material_history(),protect_project_material_identity();
COMMIT;
