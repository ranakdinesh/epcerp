CREATE TABLE supplier_document_approvals (
 document_id uuid PRIMARY KEY REFERENCES supplier_documents(id),
 supplier_id uuid NOT NULL REFERENCES suppliers(id), document_version integer NOT NULL CHECK(document_version>0),sha256 char(64) NOT NULL,
 state varchar(12) NOT NULL CHECK(state IN('pending','accept','reject')),comment text NOT NULL DEFAULT '',version integer NOT NULL CHECK(version>0),
 is_active boolean NOT NULL DEFAULT true,created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),
 created_by_id uuid REFERENCES users(id) ON DELETE SET NULL,updated_by_id uuid REFERENCES users(id) ON DELETE SET NULL
);
CREATE TABLE supplier_document_tasks (
 id uuid PRIMARY KEY,document_id uuid NOT NULL REFERENCES supplier_documents(id),supplier_id uuid NOT NULL REFERENCES suppliers(id),
 user_id uuid NOT NULL REFERENCES users(id),kind varchar(12) NOT NULL CHECK(kind IN('review','correction')),
 outcome varchar(16) NOT NULL DEFAULT '' CHECK(outcome IN('','approved','rejected','replaced','resubmitted','archived')),
 comment text NOT NULL DEFAULT '',completed_at timestamptz,
 is_active boolean NOT NULL DEFAULT true,created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),
 created_by_id uuid REFERENCES users(id) ON DELETE SET NULL,updated_by_id uuid REFERENCES users(id) ON DELETE SET NULL,
 CHECK((completed_at IS NULL AND outcome='') OR (completed_at IS NOT NULL AND outcome<>''))
);
CREATE UNIQUE INDEX supplier_document_pending_task ON supplier_document_tasks(document_id,kind) WHERE completed_at IS NULL;
CREATE INDEX supplier_document_user_tasks ON supplier_document_tasks(user_id,completed_at,created_at);
CREATE TABLE supplier_document_approval_changes (
 id uuid PRIMARY KEY,document_id uuid NOT NULL REFERENCES supplier_documents(id),version integer NOT NULL,
 action varchar(30) NOT NULL,snapshot jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),created_by_id uuid REFERENCES users(id) ON DELETE SET NULL,
 UNIQUE(document_id,version)
);
CREATE TRIGGER supplier_document_approval_changes_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON supplier_document_approval_changes FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
CREATE FUNCTION epcerp_preserve_completed_document_task() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' OR OLD.completed_at IS NOT NULL THEN RAISE EXCEPTION 'Completed document task evidence is immutable';END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER supplier_document_completed_task_immutable BEFORE UPDATE OR DELETE ON supplier_document_tasks FOR EACH ROW EXECUTE FUNCTION epcerp_preserve_completed_document_task();
