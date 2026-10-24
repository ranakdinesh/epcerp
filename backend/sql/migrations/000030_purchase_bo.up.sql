BEGIN;
CREATE TABLE purchase_bo_lists (
 id uuid PRIMARY KEY,project_id uuid NOT NULL REFERENCES projects(id),preparer_id uuid NOT NULL REFERENCES users(id),name varchar(128) NOT NULL,state varchar(20) NOT NULL CHECK(state IN('draft','completed')),is_active boolean NOT NULL,version integer NOT NULL CHECK(version>0),completed_at timestamptz,completed_by_id uuid REFERENCES users(id),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id),
 CHECK((state='completed')=(completed_at IS NOT NULL AND completed_by_id IS NOT NULL))
);
CREATE INDEX purchase_bo_project ON purchase_bo_lists(project_id);
CREATE TABLE purchase_bo_lines (
 id uuid PRIMARY KEY,bo_id uuid NOT NULL REFERENCES purchase_bo_lists(id),item_id uuid REFERENCES catalogue_items(id),quantity numeric(16,3) CHECK(quantity>0),required_date date,reference varchar(500) NOT NULL,notes varchar(4000) NOT NULL,item_snapshot jsonb NOT NULL,demand_id uuid REFERENCES project_material_requirements(id),sort_order integer NOT NULL,is_active boolean NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id)
);
CREATE TABLE purchase_bo_events (
 id uuid PRIMARY KEY,subject_id uuid NOT NULL REFERENCES purchase_bo_lists(id),action varchar(40) NOT NULL,reason varchar(2000) NOT NULL,before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id)
);
CREATE FUNCTION protect_purchase_bo_history() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'BO evidence is immutable; use controlled revisions'; END $$;
CREATE TRIGGER purchase_bo_no_delete BEFORE DELETE ON purchase_bo_lists FOR EACH ROW EXECUTE FUNCTION protect_purchase_bo_history();
CREATE TRIGGER purchase_bo_no_truncate BEFORE TRUNCATE ON purchase_bo_lists FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_bo_history();
CREATE TRIGGER purchase_bo_lines_no_delete BEFORE DELETE ON purchase_bo_lines FOR EACH ROW EXECUTE FUNCTION protect_purchase_bo_history();
CREATE TRIGGER purchase_bo_lines_no_truncate BEFORE TRUNCATE ON purchase_bo_lines FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_bo_history();
CREATE TRIGGER purchase_bo_events_immutable BEFORE UPDATE OR DELETE ON purchase_bo_events FOR EACH ROW EXECUTE FUNCTION protect_purchase_bo_history();
CREATE TRIGGER purchase_bo_events_no_truncate BEFORE TRUNCATE ON purchase_bo_events FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_bo_history();
CREATE FUNCTION protect_purchase_bo_ownership() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF ROW(NEW.project_id,NEW.preparer_id,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.project_id,OLD.preparer_id,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'BO ownership/creation evidence is immutable'; END IF;RETURN NEW;
END $$;
CREATE TRIGGER purchase_bo_ownership BEFORE UPDATE ON purchase_bo_lists FOR EACH ROW EXECUTE FUNCTION protect_purchase_bo_ownership();
CREATE FUNCTION protect_purchase_bo_line_owner() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF ROW(NEW.bo_id,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.bo_id,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'BO line ownership/creation evidence is immutable'; END IF;RETURN NEW;
END $$;
CREATE TRIGGER purchase_bo_line_owner BEFORE UPDATE ON purchase_bo_lines FOR EACH ROW EXECUTE FUNCTION protect_purchase_bo_line_owner();
COMMIT;
