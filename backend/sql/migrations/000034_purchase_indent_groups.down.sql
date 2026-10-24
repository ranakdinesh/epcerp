BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM purchase_indent_group_sets) THEN RAISE EXCEPTION 'Populated grouped-indent history cannot be removed'; END IF; END $$;
DROP TABLE purchase_indent_group_events,purchase_indent_group_entries,purchase_indent_group_sets;
DROP FUNCTION protect_indent_group_evidence(),protect_indent_group_owner();
DROP FUNCTION protect_indent_group_project();
COMMIT;
