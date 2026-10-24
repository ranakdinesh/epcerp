BEGIN;
CREATE TABLE suppliers (
 id uuid PRIMARY KEY, code varchar(32) NOT NULL UNIQUE, name varchar(255) NOT NULL,
 data jsonb NOT NULL CHECK(jsonb_typeof(data)='object'), is_active boolean NOT NULL,
 version integer NOT NULL DEFAULT 1 CHECK(version>0),
 created_at timestamptz NOT NULL DEFAULT now(), created_by_id uuid REFERENCES users(id) ON DELETE SET NULL,
 updated_at timestamptz NOT NULL DEFAULT now(), updated_by_id uuid REFERENCES users(id) ON DELETE SET NULL,
 source_id uuid NOT NULL UNIQUE, source_version integer NOT NULL CHECK(source_version>0),
 source_created_at timestamptz NOT NULL, source_updated_at timestamptz NOT NULL
);
CREATE TABLE supplier_documents (
 id uuid PRIMARY KEY, supplier_id uuid NOT NULL REFERENCES suppliers(id),
 doc_type varchar(50) NOT NULL CHECK(doc_type IN ('aadhar','pan','gst','cheque','other')),
 title varchar(255) NOT NULL, original_filename varchar(255) NOT NULL,
 content_type varchar(120) NOT NULL, byte_size integer NOT NULL CHECK(byte_size>0 AND byte_size<=15728640),
 encrypted_content bytea NOT NULL, sha256 varchar(64) NOT NULL, is_active boolean NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), created_by_id uuid REFERENCES users(id) ON DELETE SET NULL,
 updated_at timestamptz NOT NULL DEFAULT now(), updated_by_id uuid REFERENCES users(id) ON DELETE SET NULL,
 source_id uuid NOT NULL UNIQUE, source_created_at timestamptz NOT NULL, source_updated_at timestamptz NOT NULL
);
CREATE INDEX supplier_documents_supplier_idx ON supplier_documents(supplier_id,created_at,id);
COMMIT;
