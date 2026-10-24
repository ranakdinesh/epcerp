BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM client_purchase_rules) THEN RAISE EXCEPTION 'Populated client purchase policy history cannot be removed'; END IF; END $$;
DROP TABLE client_purchase_rule_events,client_purchase_rules;
DROP FUNCTION protect_client_purchase_rule_history();
COMMIT;
