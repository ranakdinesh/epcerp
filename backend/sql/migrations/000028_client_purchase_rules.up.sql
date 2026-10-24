BEGIN;
CREATE TABLE client_purchase_rules (
 id uuid PRIMARY KEY,client_id uuid NOT NULL REFERENCES clients(id),project_id uuid REFERENCES projects(id),data jsonb NOT NULL,is_active boolean NOT NULL,version integer NOT NULL CHECK(version>0),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id)
);
CREATE INDEX client_purchase_rules_client ON client_purchase_rules(client_id);
CREATE TABLE client_purchase_rule_events (
 id uuid PRIMARY KEY,subject_id uuid NOT NULL REFERENCES client_purchase_rules(id),action varchar(40) NOT NULL,reason varchar(2000) NOT NULL,before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id)
);
CREATE FUNCTION protect_client_purchase_rule_history() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Client purchase policy evidence is immutable; deactivate instead'; END $$;
CREATE TRIGGER client_purchase_rules_no_delete BEFORE DELETE ON client_purchase_rules FOR EACH ROW EXECUTE FUNCTION protect_client_purchase_rule_history();
CREATE TRIGGER client_purchase_rules_no_truncate BEFORE TRUNCATE ON client_purchase_rules FOR EACH STATEMENT EXECUTE FUNCTION protect_client_purchase_rule_history();
CREATE TRIGGER client_purchase_rule_events_immutable BEFORE UPDATE OR DELETE ON client_purchase_rule_events FOR EACH ROW EXECUTE FUNCTION protect_client_purchase_rule_history();
CREATE TRIGGER client_purchase_rule_events_no_truncate BEFORE TRUNCATE ON client_purchase_rule_events FOR EACH STATEMENT EXECUTE FUNCTION protect_client_purchase_rule_history();
COMMIT;
