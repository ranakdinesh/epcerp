BEGIN;
CREATE TABLE project_material_requirements (
 id uuid PRIMARY KEY,project_id uuid NOT NULL REFERENCES projects(id),item_id uuid NOT NULL REFERENCES catalogue_items(id),engineering_key varchar(64) NOT NULL,item_snapshot jsonb NOT NULL,
 quantity numeric(16,3) NOT NULL CHECK(quantity>0),required_date date NOT NULL,allocations jsonb NOT NULL,alternatives jsonb NOT NULL,is_active boolean NOT NULL,version integer NOT NULL CHECK(version>0),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id),
 UNIQUE(project_id,item_id),UNIQUE(project_id,engineering_key)
);
CREATE TABLE project_material_events(
 id uuid PRIMARY KEY,subject_id uuid NOT NULL REFERENCES project_material_requirements(id),action varchar(40) NOT NULL,reason varchar(2000) NOT NULL,before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id)
);
CREATE FUNCTION protect_project_material_history() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Project material history is immutable; deactivate instead'; END $$;
CREATE TRIGGER project_material_no_delete BEFORE DELETE ON project_material_requirements FOR EACH ROW EXECUTE FUNCTION protect_project_material_history();
CREATE TRIGGER project_material_no_truncate BEFORE TRUNCATE ON project_material_requirements FOR EACH STATEMENT EXECUTE FUNCTION protect_project_material_history();
CREATE TRIGGER project_material_events_immutable BEFORE UPDATE OR DELETE ON project_material_events FOR EACH ROW EXECUTE FUNCTION protect_project_material_history();
CREATE TRIGGER project_material_events_no_truncate BEFORE TRUNCATE ON project_material_events FOR EACH STATEMENT EXECUTE FUNCTION protect_project_material_history();
CREATE FUNCTION protect_project_material_identity() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF ROW(NEW.project_id,NEW.item_id,NEW.engineering_key,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.project_id,OLD.item_id,OLD.engineering_key,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Project material identity cannot be replaced'; END IF;RETURN NEW;
END $$;
CREATE TRIGGER project_material_identity BEFORE UPDATE ON project_material_requirements FOR EACH ROW EXECUTE FUNCTION protect_project_material_identity();
COMMIT;
