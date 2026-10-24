BEGIN;
CREATE TABLE purchase_group_quotations (
 bo_id uuid PRIMARY KEY REFERENCES purchase_bo_lists(id), project_id uuid NOT NULL REFERENCES projects(id),version integer NOT NULL CHECK(version>0),quotes jsonb NOT NULL CHECK(jsonb_typeof(quotes)='array'),is_active boolean NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id)
);
CREATE TABLE purchase_quotation_attachments (
 id uuid PRIMARY KEY,document_id uuid NOT NULL REFERENCES purchase_indent_documents(id),filename varchar(255) NOT NULL,content_type varchar(128) NOT NULL,byte_size bigint NOT NULL CHECK(byte_size>0 AND byte_size<=10485760),sha256 char(64) NOT NULL CHECK(sha256 ~ '^[0-9a-f]{64}$'),ciphertext bytea NOT NULL,is_active boolean NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id),UNIQUE(document_id,id)
);
CREATE TABLE purchase_quotation_revisions (
 document_id uuid NOT NULL REFERENCES purchase_indent_documents(id),revision integer NOT NULL CHECK(revision>0),bo_id uuid NOT NULL REFERENCES purchase_group_quotations(bo_id),attachment_id uuid,quote jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),PRIMARY KEY(document_id,revision),FOREIGN KEY(document_id,attachment_id) REFERENCES purchase_quotation_attachments(document_id,id),UNIQUE(attachment_id)
);
CREATE TABLE purchase_quotation_events (
 id uuid PRIMARY KEY,bo_id uuid NOT NULL REFERENCES purchase_group_quotations(bo_id),action varchar(40) NOT NULL,reason varchar(2000) NOT NULL,before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id)
);
CREATE TABLE purchase_quotation_evidence_events (
 id uuid PRIMARY KEY,attachment_id uuid NOT NULL REFERENCES purchase_quotation_attachments(id),action varchar(40) NOT NULL CHECK(action IN ('uploaded','downloaded')),created_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id)
);
CREATE INDEX purchase_quotation_project ON purchase_group_quotations(project_id,bo_id);
CREATE INDEX purchase_quotation_attachment_document ON purchase_quotation_attachments(document_id,created_at);
CREATE FUNCTION protect_quotation_evidence() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Quotation evidence and revisions are immutable';END $$;
CREATE TRIGGER purchase_quotation_attachments_immutable BEFORE UPDATE OR DELETE ON purchase_quotation_attachments FOR EACH ROW EXECUTE FUNCTION protect_quotation_evidence();
CREATE TRIGGER purchase_quotation_attachments_no_truncate BEFORE TRUNCATE ON purchase_quotation_attachments FOR EACH STATEMENT EXECUTE FUNCTION protect_quotation_evidence();
CREATE TRIGGER purchase_quotation_revisions_immutable BEFORE UPDATE OR DELETE ON purchase_quotation_revisions FOR EACH ROW EXECUTE FUNCTION protect_quotation_evidence();
CREATE TRIGGER purchase_quotation_revisions_no_truncate BEFORE TRUNCATE ON purchase_quotation_revisions FOR EACH STATEMENT EXECUTE FUNCTION protect_quotation_evidence();
CREATE TRIGGER purchase_quotation_events_immutable BEFORE UPDATE OR DELETE ON purchase_quotation_events FOR EACH ROW EXECUTE FUNCTION protect_quotation_evidence();
CREATE TRIGGER purchase_quotation_events_no_truncate BEFORE TRUNCATE ON purchase_quotation_events FOR EACH STATEMENT EXECUTE FUNCTION protect_quotation_evidence();
CREATE TRIGGER purchase_quotation_evidence_events_immutable BEFORE UPDATE OR DELETE ON purchase_quotation_evidence_events FOR EACH ROW EXECUTE FUNCTION protect_quotation_evidence();
CREATE TRIGGER purchase_quotation_evidence_events_no_truncate BEFORE TRUNCATE ON purchase_quotation_evidence_events FOR EACH STATEMENT EXECUTE FUNCTION protect_quotation_evidence();
CREATE TRIGGER purchase_group_quotations_no_delete BEFORE DELETE ON purchase_group_quotations FOR EACH ROW EXECUTE FUNCTION protect_quotation_evidence();
CREATE TRIGGER purchase_group_quotations_no_truncate BEFORE TRUNCATE ON purchase_group_quotations FOR EACH STATEMENT EXECUTE FUNCTION protect_quotation_evidence();
CREATE FUNCTION protect_quotation_owner() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF TG_OP='UPDATE' AND ROW(NEW.bo_id,NEW.project_id,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.bo_id,OLD.project_id,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Quotation BO ownership is immutable';END IF;
 IF NOT EXISTS(SELECT 1 FROM purchase_bo_lists WHERE id=NEW.bo_id AND project_id=NEW.project_id) THEN RAISE EXCEPTION 'Quotation BO belongs to another project';END IF;RETURN NEW;END $$;
CREATE TRIGGER purchase_group_quotation_owner BEFORE INSERT OR UPDATE ON purchase_group_quotations FOR EACH ROW EXECUTE FUNCTION protect_quotation_owner();
CREATE FUNCTION protect_quotation_revision_scope() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_indent_documents WHERE id=NEW.document_id AND bo_id=NEW.bo_id) THEN RAISE EXCEPTION 'Quotation document belongs to another BO';END IF;RETURN NEW;END $$;
CREATE TRIGGER purchase_quotation_revision_scope BEFORE INSERT ON purchase_quotation_revisions FOR EACH ROW EXECUTE FUNCTION protect_quotation_revision_scope();
COMMIT;
