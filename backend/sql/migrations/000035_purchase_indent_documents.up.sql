BEGIN;
CREATE TABLE purchase_indent_documents (
 id uuid PRIMARY KEY,project_id uuid NOT NULL REFERENCES projects(id),bo_id uuid NOT NULL REFERENCES purchase_bo_lists(id),group_id uuid NOT NULL REFERENCES purchase_indent_group_entries(id),vendor_id uuid NOT NULL REFERENCES suppliers(id),scope_revision integer NOT NULL CHECK(scope_revision>0),version integer NOT NULL CHECK(version>0),data jsonb NOT NULL,is_active boolean NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id),UNIQUE(group_id,vendor_id,scope_revision)
);
CREATE INDEX purchase_indent_document_scope ON purchase_indent_documents(project_id,bo_id);
CREATE TABLE purchase_indent_document_events(id uuid PRIMARY KEY,document_id uuid NOT NULL REFERENCES purchase_indent_documents(id),action varchar(40) NOT NULL,reason varchar(2000) NOT NULL,before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id));
CREATE TABLE purchase_indent_dispatch_notices(id uuid PRIMARY KEY,document_id uuid NOT NULL REFERENCES purchase_indent_documents(id),recipient_id uuid NOT NULL REFERENCES users(id),created_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),resolved_at timestamptz,updated_by_id uuid REFERENCES users(id),UNIQUE(document_id,recipient_id));
CREATE INDEX purchase_indent_notice_recipient ON purchase_indent_dispatch_notices(recipient_id,created_at DESC) WHERE resolved_at IS NULL;
CREATE FUNCTION protect_indent_document_evidence() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Vendor indent evidence cannot be deleted or rewritten'; END $$;
CREATE TRIGGER purchase_indent_documents_no_delete BEFORE DELETE ON purchase_indent_documents FOR EACH ROW EXECUTE FUNCTION protect_indent_document_evidence();
CREATE TRIGGER purchase_indent_documents_no_truncate BEFORE TRUNCATE ON purchase_indent_documents FOR EACH STATEMENT EXECUTE FUNCTION protect_indent_document_evidence();
CREATE TRIGGER purchase_indent_document_events_immutable BEFORE UPDATE OR DELETE ON purchase_indent_document_events FOR EACH ROW EXECUTE FUNCTION protect_indent_document_evidence();
CREATE TRIGGER purchase_indent_document_events_no_truncate BEFORE TRUNCATE ON purchase_indent_document_events FOR EACH STATEMENT EXECUTE FUNCTION protect_indent_document_evidence();
CREATE TRIGGER purchase_indent_notices_no_delete BEFORE DELETE ON purchase_indent_dispatch_notices FOR EACH ROW EXECUTE FUNCTION protect_indent_document_evidence();
CREATE TRIGGER purchase_indent_notices_no_truncate BEFORE TRUNCATE ON purchase_indent_dispatch_notices FOR EACH STATEMENT EXECUTE FUNCTION protect_indent_document_evidence();
CREATE FUNCTION protect_indent_document_scope() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
IF ROW(NEW.id,NEW.project_id,NEW.bo_id,NEW.group_id,NEW.vendor_id,NEW.scope_revision,NEW.created_at,NEW.created_by_id, NEW.data-'print_requests'-'sent_date'-'channel'-'dispatch_reference'-'sent_recorded_at'-'sent_by_id') IS DISTINCT FROM ROW(OLD.id,OLD.project_id,OLD.bo_id,OLD.group_id,OLD.vendor_id,OLD.scope_revision,OLD.created_at,OLD.created_by_id,OLD.data-'print_requests'-'sent_date'-'channel'-'dispatch_reference'-'sent_recorded_at'-'sent_by_id') THEN RAISE EXCEPTION 'Generated vendor indent scope is immutable'; END IF;
IF OLD.data->>'sent_date' IS NOT NULL AND ROW(NEW.data->'sent_date',NEW.data->'channel',NEW.data->'dispatch_reference',NEW.data->'sent_recorded_at',NEW.data->'sent_by_id') IS DISTINCT FROM ROW(OLD.data->'sent_date',OLD.data->'channel',OLD.data->'dispatch_reference',OLD.data->'sent_recorded_at',OLD.data->'sent_by_id') THEN RAISE EXCEPTION 'Recorded dispatch is immutable';END IF;
RETURN NEW; END $$;
CREATE TRIGGER purchase_indent_document_scope BEFORE UPDATE ON purchase_indent_documents FOR EACH ROW EXECUTE FUNCTION protect_indent_document_scope();
CREATE FUNCTION protect_indent_notice_owner() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF ROW(NEW.id,NEW.document_id,NEW.recipient_id,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.id,OLD.document_id,OLD.recipient_id,OLD.created_at,OLD.created_by_id) OR OLD.resolved_at IS NOT NULL THEN RAISE EXCEPTION 'Dispatch notice identity and resolution are immutable';END IF;RETURN NEW;END $$;
CREATE TRIGGER purchase_indent_notice_owner BEFORE UPDATE ON purchase_indent_dispatch_notices FOR EACH ROW EXECUTE FUNCTION protect_indent_notice_owner();
COMMIT;
