CREATE TABLE purchase_fms_workspaces (
 project_id uuid PRIMARY KEY REFERENCES projects(id),
 purchase_manager_id uuid REFERENCES users(id),
 version integer NOT NULL DEFAULT 1 CHECK(version>0),
 is_active boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 created_by_id uuid REFERENCES users(id),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 updated_by_id uuid REFERENCES users(id)
);
CREATE INDEX purchase_fms_owner_projects ON purchase_fms_workspaces(purchase_manager_id,project_id) WHERE is_active;
CREATE TABLE purchase_fms_owner_history (
 id uuid PRIMARY KEY,
 project_id uuid NOT NULL REFERENCES purchase_fms_workspaces(project_id),
 previous_manager_id uuid REFERENCES users(id),
 purchase_manager_id uuid NOT NULL REFERENCES users(id),
 workspace_version integer NOT NULL CHECK(workspace_version>0),
 reason text NOT NULL CHECK(length(trim(reason)) BETWEEN 1 AND 2000),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 created_by_id uuid REFERENCES users(id),
 UNIQUE(project_id,workspace_version)
);
CREATE FUNCTION purchase_fms_history_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'Purchase ownership history is immutable'; END; $$;
CREATE TRIGGER purchase_fms_history_immutable BEFORE UPDATE OR DELETE ON purchase_fms_owner_history FOR EACH ROW EXECUTE FUNCTION purchase_fms_history_immutable();

CREATE TRIGGER purchase_fms_history_no_truncate BEFORE TRUNCATE ON purchase_fms_owner_history FOR EACH STATEMENT EXECUTE FUNCTION purchase_fms_history_immutable();
