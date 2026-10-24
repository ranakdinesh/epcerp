BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM purchase_holds) THEN RAISE EXCEPTION 'Populated purchase hold history cannot be removed'; END IF; END $$;
DROP TABLE purchase_hold_events,purchase_holds;
DROP FUNCTION protect_purchase_hold_history();
DROP FUNCTION protect_purchase_hold_scope();
COMMIT;
