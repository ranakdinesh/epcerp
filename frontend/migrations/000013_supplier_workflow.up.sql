BEGIN;
ALTER TABLE suppliers ALTER COLUMN source_id DROP NOT NULL, ALTER COLUMN source_version DROP NOT NULL, ALTER COLUMN source_created_at DROP NOT NULL, ALTER COLUMN source_updated_at DROP NOT NULL;
ALTER TABLE supplier_documents ALTER COLUMN source_id DROP NOT NULL, ALTER COLUMN source_created_at DROP NOT NULL, ALTER COLUMN source_updated_at DROP NOT NULL;
ALTER TABLE supplier_documents ADD COLUMN version integer NOT NULL DEFAULT 1 CHECK(version>0);
CREATE TABLE supplier_verifications (
 supplier_id uuid PRIMARY KEY REFERENCES suppliers(id), version integer NOT NULL CHECK(version>0),
 fingerprint varchar(64) NOT NULL, decisions jsonb NOT NULL CHECK(jsonb_typeof(decisions)='array'), reason varchar(2000) NOT NULL,
 is_active boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT now(), created_by_id uuid REFERENCES users(id) ON DELETE SET NULL,
 updated_at timestamptz NOT NULL DEFAULT now(), updated_by_id uuid REFERENCES users(id) ON DELETE SET NULL
);
CREATE TABLE supplier_changes (id uuid PRIMARY KEY,supplier_id uuid NOT NULL REFERENCES suppliers(id),version integer NOT NULL,action varchar(50) NOT NULL,reason varchar(2000) NOT NULL,snapshot jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),created_by_id uuid REFERENCES users(id) ON DELETE SET NULL,UNIQUE(supplier_id,version));
CREATE TABLE supplier_document_changes (id uuid PRIMARY KEY,document_id uuid NOT NULL REFERENCES supplier_documents(id),version integer NOT NULL,action varchar(50) NOT NULL,reason varchar(2000) NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),created_by_id uuid REFERENCES users(id) ON DELETE SET NULL,UNIQUE(document_id,version));
CREATE TABLE supplier_verification_history (id uuid PRIMARY KEY,supplier_id uuid NOT NULL REFERENCES suppliers(id),version integer NOT NULL,snapshot jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),created_by_id uuid REFERENCES users(id) ON DELETE SET NULL,UNIQUE(supplier_id,version));
CREATE TRIGGER supplier_changes_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON supplier_changes FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
CREATE TRIGGER supplier_document_changes_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON supplier_document_changes FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
CREATE TRIGGER supplier_verification_history_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON supplier_verification_history FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
COMMIT;
