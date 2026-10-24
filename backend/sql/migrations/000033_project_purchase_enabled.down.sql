BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM purchase_bo_lists) OR EXISTS(SELECT 1 FROM purchase_holds) OR EXISTS(SELECT 1 FROM purchase_indents) THEN RAISE EXCEPTION 'Purchase availability cannot be removed after workflow evidence exists'; END IF; END $$;
ALTER TABLE projects DROP COLUMN purchase_enabled;
COMMIT;
