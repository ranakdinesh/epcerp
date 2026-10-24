BEGIN;
CREATE TABLE project_responsibility_profiles (
 id uuid PRIMARY KEY, project_id uuid NOT NULL REFERENCES projects(id), user_id uuid NOT NULL REFERENCES users(id), bundle_id uuid NOT NULL REFERENCES roles(id),
 responsibility varchar(80) NOT NULL CHECK(length(btrim(responsibility))>0), responsibility_key varchar(80) NOT NULL,
 permission_codes jsonb NOT NULL CHECK(jsonb_typeof(permission_codes)='array' AND jsonb_array_length(permission_codes) BETWEEN 1 AND 100),
 is_active boolean NOT NULL DEFAULT true, version integer NOT NULL CHECK(version>0),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id), UNIQUE(id,project_id)
);
CREATE UNIQUE INDEX project_responsibility_active_identity ON project_responsibility_profiles(project_id,user_id,bundle_id,responsibility_key) WHERE is_active;
CREATE INDEX project_responsibility_scope ON project_responsibility_profiles(project_id,bundle_id,id);
CREATE TABLE project_responsibility_events (
 id uuid PRIMARY KEY,profile_id uuid NOT NULL,project_id uuid NOT NULL REFERENCES projects(id),action varchar(20) NOT NULL CHECK(action IN('assign','revise','revoke')),
 reason varchar(2000) NOT NULL CHECK(length(btrim(reason))>0),before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),FOREIGN KEY(profile_id,project_id) REFERENCES project_responsibility_profiles(id,project_id)
);
CREATE INDEX project_responsibility_history ON project_responsibility_events(profile_id,created_at,id);
CREATE FUNCTION protect_project_responsibility_evidence() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Project responsibility evidence is immutable; deactivate instead'; END $$;
CREATE TRIGGER project_responsibility_no_delete BEFORE DELETE ON project_responsibility_profiles FOR EACH ROW EXECUTE FUNCTION protect_project_responsibility_evidence();
CREATE TRIGGER project_responsibility_no_truncate BEFORE TRUNCATE ON project_responsibility_profiles FOR EACH STATEMENT EXECUTE FUNCTION protect_project_responsibility_evidence();
CREATE TRIGGER project_responsibility_events_immutable BEFORE UPDATE OR DELETE ON project_responsibility_events FOR EACH ROW EXECUTE FUNCTION protect_project_responsibility_evidence();
CREATE TRIGGER project_responsibility_events_no_truncate BEFORE TRUNCATE ON project_responsibility_events FOR EACH STATEMENT EXECUTE FUNCTION protect_project_responsibility_evidence();
CREATE FUNCTION protect_project_responsibility_scope() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF ROW(NEW.id,NEW.project_id,NEW.user_id,NEW.bundle_id,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.id,OLD.project_id,OLD.user_id,OLD.bundle_id,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Project responsibility identity is immutable';END IF;
 IF NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'Project responsibility requires the next version';END IF;
 RETURN NEW;END $$;
CREATE TRIGGER project_responsibility_scope BEFORE UPDATE ON project_responsibility_profiles FOR EACH ROW EXECUTE FUNCTION protect_project_responsibility_scope();
COMMIT;
