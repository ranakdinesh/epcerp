BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM purchase_indents) THEN RAISE EXCEPTION 'Populated purchase hold history cannot be removed'; END IF; END $$;
DROP TABLE purchase_indent_releases,purchase_indent_events,purchase_indents;
DROP FUNCTION protect_purchase_indent_history();
DROP FUNCTION protect_purchase_indent_scope();
COMMIT;
