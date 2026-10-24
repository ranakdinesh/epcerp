BEGIN;
CREATE TABLE purchase_indent_group_sets (
 bo_id uuid PRIMARY KEY REFERENCES purchase_bo_lists(id),project_id uuid NOT NULL REFERENCES projects(id),version integer NOT NULL CHECK(version>0),data jsonb NOT NULL,is_active boolean NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id)
);
CREATE INDEX purchase_indent_group_project ON purchase_indent_group_sets(project_id);
CREATE TABLE purchase_indent_group_entries (
 id uuid PRIMARY KEY,bo_id uuid NOT NULL REFERENCES purchase_indent_group_sets(bo_id),name varchar(128) NOT NULL,scope_revision integer NOT NULL CHECK(scope_revision>0),data jsonb NOT NULL,is_active boolean NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id)
);
CREATE TABLE purchase_indent_group_events (
 id uuid PRIMARY KEY,bo_id uuid NOT NULL REFERENCES purchase_indent_group_sets(bo_id),action varchar(40) NOT NULL,reason varchar(2000) NOT NULL,before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id)
);
CREATE FUNCTION protect_indent_group_evidence() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Grouped indent evidence cannot be deleted or rewritten'; END $$;
CREATE TRIGGER purchase_indent_groups_no_delete BEFORE DELETE ON purchase_indent_group_sets FOR EACH ROW EXECUTE FUNCTION protect_indent_group_evidence();
CREATE TRIGGER purchase_indent_groups_no_truncate BEFORE TRUNCATE ON purchase_indent_group_sets FOR EACH STATEMENT EXECUTE FUNCTION protect_indent_group_evidence();
CREATE TRIGGER purchase_indent_group_entries_no_delete BEFORE DELETE ON purchase_indent_group_entries FOR EACH ROW EXECUTE FUNCTION protect_indent_group_evidence();
CREATE TRIGGER purchase_indent_group_entries_no_truncate BEFORE TRUNCATE ON purchase_indent_group_entries FOR EACH STATEMENT EXECUTE FUNCTION protect_indent_group_evidence();
CREATE TRIGGER purchase_indent_group_events_immutable BEFORE UPDATE OR DELETE ON purchase_indent_group_events FOR EACH ROW EXECUTE FUNCTION protect_indent_group_evidence();
CREATE TRIGGER purchase_indent_group_events_no_truncate BEFORE TRUNCATE ON purchase_indent_group_events FOR EACH STATEMENT EXECUTE FUNCTION protect_indent_group_evidence();
CREATE FUNCTION protect_indent_group_owner() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF ROW(NEW.bo_id,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.bo_id,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Grouped indent identity and creation evidence are immutable'; END IF;RETURN NEW; END $$;
CREATE TRIGGER purchase_indent_group_owner BEFORE UPDATE ON purchase_indent_group_sets FOR EACH ROW EXECUTE FUNCTION protect_indent_group_owner();
CREATE TRIGGER purchase_indent_group_entry_owner BEFORE UPDATE ON purchase_indent_group_entries FOR EACH ROW EXECUTE FUNCTION protect_indent_group_owner();
CREATE FUNCTION protect_indent_group_project() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.project_id IS DISTINCT FROM OLD.project_id THEN RAISE EXCEPTION 'Grouped indent project ownership is immutable';END IF;RETURN NEW;END $$;
CREATE TRIGGER purchase_indent_group_project_owner BEFORE UPDATE ON purchase_indent_group_sets FOR EACH ROW EXECUTE FUNCTION protect_indent_group_project();
COMMIT;
