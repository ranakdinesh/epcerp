BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM quotation_approval_sets) OR EXISTS(SELECT 1 FROM quotation_approval_events) OR EXISTS(SELECT 1 FROM finance_quotation_decisions) THEN RAISE EXCEPTION 'Refusing to remove populated quotation approval/Finance evidence';END IF;END $$;
DROP TRIGGER quotation_approval_finance ON quotation_approval_sets;
DROP FUNCTION check_quotation_approval_decisions();
DROP TABLE finance_quotation_decisions;
DROP FUNCTION check_finance_quotation_event();
DROP TABLE quotation_approval_events,quotation_approval_sets;
DROP FUNCTION protect_quotation_approval_scope(),protect_quotation_approval_history();
COMMIT;
