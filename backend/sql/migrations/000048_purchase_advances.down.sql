BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM purchase_advances) OR EXISTS(SELECT 1 FROM finance_advance_decisions) THEN RAISE EXCEPTION 'Cannot rollback populated purchase advances';END IF;END $$;
ALTER TABLE purchase_advances DROP CONSTRAINT purchase_advance_decision_fk;
DROP TABLE finance_advance_decisions,purchase_advance_events,purchase_advances;
DROP FUNCTION protect_purchase_advance(),protect_purchase_advance_history(),check_purchase_advance_write(),check_purchase_advance_event(),protect_finance_advance_source(),check_finance_advance_event();
COMMIT;
