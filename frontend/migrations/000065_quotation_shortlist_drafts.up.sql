BEGIN;
CREATE TABLE purchase_quotation_shortlist_drafts (
 project_id uuid NOT NULL REFERENCES projects(id),
 bo_id uuid NOT NULL REFERENCES purchase_bo_lists(id),
 group_id uuid NOT NULL REFERENCES purchase_indent_group_entries(id),
 version integer NOT NULL CHECK(version>0),
 proposals jsonb NOT NULL CHECK(jsonb_typeof(proposals)='array' AND jsonb_array_length(proposals) BETWEEN 1 AND 2),
 comment varchar(2000) NOT NULL DEFAULT '',
 is_active boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id),
 PRIMARY KEY(project_id,bo_id,group_id)
);
CREATE TABLE purchase_quotation_shortlist_history (
 id uuid PRIMARY KEY,project_id uuid NOT NULL REFERENCES projects(id),bo_id uuid NOT NULL REFERENCES purchase_bo_lists(id),group_id uuid NOT NULL REFERENCES purchase_indent_group_entries(id),version integer NOT NULL,
 action text NOT NULL CHECK(action IN('saved','submitted')),snapshot jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id)
);
CREATE TRIGGER purchase_quotation_shortlist_history_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON purchase_quotation_shortlist_history FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
CREATE TRIGGER purchase_quotation_shortlist_no_delete BEFORE DELETE OR TRUNCATE ON purchase_quotation_shortlist_drafts FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
COMMIT;
