BEGIN;
CREATE TABLE purchase_holds (
 id uuid PRIMARY KEY,project_id uuid NOT NULL REFERENCES projects(id),doer_id uuid NOT NULL REFERENCES users(id),resolver_id uuid NOT NULL REFERENCES users(id),data jsonb NOT NULL,is_active boolean NOT NULL,version integer NOT NULL CHECK(version>0),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id)
);
CREATE INDEX purchase_holds_project ON purchase_holds(project_id);
CREATE TABLE purchase_hold_events (
 id uuid PRIMARY KEY,subject_id uuid NOT NULL REFERENCES purchase_holds(id),action varchar(40) NOT NULL,reason varchar(2000) NOT NULL,before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id)
);
CREATE FUNCTION protect_purchase_hold_history() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Purchase hold evidence is immutable; deactivate instead'; END $$;
CREATE TRIGGER purchase_holds_no_delete BEFORE DELETE ON purchase_holds FOR EACH ROW EXECUTE FUNCTION protect_purchase_hold_history();
CREATE TRIGGER purchase_holds_no_truncate BEFORE TRUNCATE ON purchase_holds FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_hold_history();
CREATE TRIGGER purchase_hold_events_immutable BEFORE UPDATE OR DELETE ON purchase_hold_events FOR EACH ROW EXECUTE FUNCTION protect_purchase_hold_history();
CREATE TRIGGER purchase_hold_events_no_truncate BEFORE TRUNCATE ON purchase_hold_events FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_hold_history();
CREATE FUNCTION protect_purchase_hold_scope() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF ROW(NEW.project_id,NEW.doer_id,NEW.resolver_id,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.project_id,OLD.doer_id,OLD.resolver_id,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Hold scope and assignment evidence are immutable'; END IF;RETURN NEW; END $$;
CREATE TRIGGER purchase_hold_scope BEFORE UPDATE ON purchase_holds FOR EACH ROW EXECUTE FUNCTION protect_purchase_hold_scope();
COMMIT;
