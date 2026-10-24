BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM purchase_bo_lists) THEN RAISE EXCEPTION 'Populated BO history cannot be removed'; END IF; END $$;
DROP TABLE purchase_bo_events,purchase_bo_lines,purchase_bo_lists;
DROP FUNCTION protect_purchase_bo_history(),protect_purchase_bo_ownership(),protect_purchase_bo_line_owner();
COMMIT;
