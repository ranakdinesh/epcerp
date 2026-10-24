BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM purchase_indent_legacy_terms) THEN RAISE EXCEPTION 'Legacy indent terms are immutable evidence; retain this migration while issued documents exist'; END IF; END $$;
DROP TABLE purchase_indent_legacy_terms;
COMMIT;
