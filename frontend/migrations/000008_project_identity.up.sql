BEGIN;
CREATE SEQUENCE project_number_sequence;
CREATE TABLE projects (
 id UUID PRIMARY KEY,
 project_number VARCHAR(32) NOT NULL UNIQUE,
 name VARCHAR(255) NOT NULL,
 normalized_name VARCHAR(255) NOT NULL UNIQUE,
 client_id UUID NOT NULL REFERENCES clients(id),
 project_manager_id UUID NOT NULL REFERENCES users(id),
 lifecycle_status VARCHAR(16) NOT NULL DEFAULT 'draft' CHECK(lifecycle_status IN ('draft','active','on_hold','closed')),
 is_active BOOLEAN NOT NULL DEFAULT true,
 version INTEGER NOT NULL DEFAULT 1 CHECK(version>0),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 created_by_id UUID REFERENCES users(id),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 updated_by_id UUID REFERENCES users(id)
);
-- Many projects may share the same manager. This index is intentionally not unique.
CREATE INDEX projects_manager_idx ON projects(project_manager_id,created_at DESC,id);
CREATE INDEX projects_client_idx ON projects(client_id);
CREATE INDEX projects_created_actor_idx ON projects(created_by_id) WHERE created_by_id IS NOT NULL;
CREATE INDEX projects_updated_actor_idx ON projects(updated_by_id) WHERE updated_by_id IS NOT NULL;
CREATE TABLE project_manager_history (
 id UUID PRIMARY KEY,
 project_id UUID NOT NULL REFERENCES projects(id),
 previous_manager_id UUID REFERENCES users(id),
 project_manager_id UUID NOT NULL REFERENCES users(id),
 project_version INTEGER NOT NULL CHECK(project_version>0),
 reason VARCHAR(2000) NOT NULL CHECK(length(btrim(reason))>0),
 created_at TIMESTAMPTZ NOT NULL,
 created_by_id UUID REFERENCES users(id),
 CHECK(previous_manager_id IS NULL OR previous_manager_id<>project_manager_id),
 UNIQUE(project_id,project_version)
);
CREATE INDEX project_manager_history_project_idx ON project_manager_history(project_id,project_version DESC);
CREATE INDEX project_manager_history_previous_idx ON project_manager_history(previous_manager_id) WHERE previous_manager_id IS NOT NULL;
CREATE INDEX project_manager_history_manager_idx ON project_manager_history(project_manager_id);
CREATE INDEX project_manager_history_actor_idx ON project_manager_history(created_by_id) WHERE created_by_id IS NOT NULL;
CREATE TRIGGER project_manager_history_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON project_manager_history FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
COMMIT;
