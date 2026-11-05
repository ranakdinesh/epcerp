BEGIN;
CREATE TABLE purchase_quotation_work_tasks(
 id uuid PRIMARY KEY,project_id uuid NOT NULL REFERENCES projects(id),bo_id uuid NOT NULL REFERENCES purchase_bo_lists(id),group_id uuid NOT NULL REFERENCES purchase_indent_group_entries(id),attempt_id uuid NOT NULL,recipient_id uuid NOT NULL REFERENCES users(id),kind text NOT NULL CHECK(kind IN('finance','executive')),title varchar(300) NOT NULL,link varchar(400) NOT NULL CHECK(link LIKE '/dashboard?%'),state text NOT NULL CHECK(state IN('pending','waiting','negotiate','change_vendor','completed')),is_active boolean NOT NULL DEFAULT true,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id),UNIQUE(attempt_id,recipient_id,kind)
);
CREATE INDEX purchase_quotation_work_recipient ON purchase_quotation_work_tasks(recipient_id,created_at DESC) WHERE state<>'completed';
CREATE TRIGGER purchase_quotation_work_no_delete BEFORE DELETE OR TRUNCATE ON purchase_quotation_work_tasks FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
COMMIT;
