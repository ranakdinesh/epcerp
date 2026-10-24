CREATE TABLE project_metadata_changes (
 id uuid PRIMARY KEY,
 project_id uuid NOT NULL REFERENCES projects(id),
 project_version integer NOT NULL CHECK(project_version>1),
 reason text NOT NULL CHECK(length(trim(reason)) BETWEEN 1 AND 2000),
 before_snapshot jsonb NOT NULL,
 after_snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 created_by_id uuid REFERENCES users(id),
 UNIQUE(project_id,project_version)
);
CREATE FUNCTION reject_project_metadata_change() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Project metadata history is immutable'; END $$;
CREATE TRIGGER project_metadata_immutable BEFORE UPDATE OR DELETE ON project_metadata_changes FOR EACH ROW EXECUTE FUNCTION reject_project_metadata_change();
CREATE TRIGGER project_metadata_no_truncate BEFORE TRUNCATE ON project_metadata_changes FOR EACH STATEMENT EXECUTE FUNCTION reject_project_metadata_change();
