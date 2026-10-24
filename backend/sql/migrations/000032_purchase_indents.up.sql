BEGIN;
CREATE TABLE purchase_indents (
 id uuid PRIMARY KEY,project_id uuid NOT NULL REFERENCES projects(id),doer_id uuid NOT NULL REFERENCES users(id),resolver_id uuid NOT NULL REFERENCES users(id),data jsonb NOT NULL,is_active boolean NOT NULL,version integer NOT NULL CHECK(version>0),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id)
);
CREATE INDEX purchase_indents_project ON purchase_indents(project_id);
CREATE TABLE purchase_indent_events (
 id uuid PRIMARY KEY,subject_id uuid NOT NULL REFERENCES purchase_indents(id),action varchar(40) NOT NULL,reason varchar(2000) NOT NULL,before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id)
);
CREATE FUNCTION protect_purchase_indent_history() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Purchase indent evidence is immutable; deactivate instead'; END $$;
CREATE TRIGGER purchase_indents_no_delete BEFORE DELETE ON purchase_indents FOR EACH ROW EXECUTE FUNCTION protect_purchase_indent_history();
CREATE TRIGGER purchase_indents_no_truncate BEFORE TRUNCATE ON purchase_indents FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_indent_history();
CREATE TRIGGER purchase_indent_events_immutable BEFORE UPDATE OR DELETE ON purchase_indent_events FOR EACH ROW EXECUTE FUNCTION protect_purchase_indent_history();
CREATE TRIGGER purchase_indent_events_no_truncate BEFORE TRUNCATE ON purchase_indent_events FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_indent_history();
CREATE FUNCTION protect_purchase_indent_scope() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF ROW(NEW.project_id,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.project_id,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Indent scope evidence are immutable'; END IF;RETURN NEW; END $$;
CREATE TRIGGER purchase_indent_scope BEFORE UPDATE ON purchase_indents FOR EACH ROW EXECUTE FUNCTION protect_purchase_indent_scope();
CREATE TABLE purchase_indent_releases (
 indent_id uuid NOT NULL REFERENCES purchase_indents(id),demand_id uuid NOT NULL REFERENCES project_material_requirements(id),quantity numeric(16,3) NOT NULL CHECK(quantity>0),snapshot jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),PRIMARY KEY(indent_id,demand_id)
);
CREATE TRIGGER purchase_indent_release_immutable BEFORE UPDATE OR DELETE ON purchase_indent_releases FOR EACH ROW EXECUTE FUNCTION protect_purchase_indent_history();
CREATE TRIGGER purchase_indent_release_no_truncate BEFORE TRUNCATE ON purchase_indent_releases FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_indent_history();
COMMIT;
