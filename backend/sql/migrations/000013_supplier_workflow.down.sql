BEGIN;
-- Native records cannot acquire invented legacy provenance. Refuse rollback when present.
DO $$ BEGIN IF EXISTS(SELECT 1 FROM suppliers WHERE source_id IS NULL) OR EXISTS(SELECT 1 FROM supplier_documents WHERE source_id IS NULL) THEN RAISE EXCEPTION 'Native supplier records prevent rollback'; END IF; END $$;
DROP TABLE supplier_verification_history, supplier_document_changes, supplier_changes, supplier_verifications;
ALTER TABLE supplier_documents DROP COLUMN version;
ALTER TABLE suppliers ALTER COLUMN source_id SET NOT NULL, ALTER COLUMN source_version SET NOT NULL, ALTER COLUMN source_created_at SET NOT NULL, ALTER COLUMN source_updated_at SET NOT NULL;
ALTER TABLE supplier_documents ALTER COLUMN source_id SET NOT NULL, ALTER COLUMN source_created_at SET NOT NULL, ALTER COLUMN source_updated_at SET NOT NULL;
COMMIT;
