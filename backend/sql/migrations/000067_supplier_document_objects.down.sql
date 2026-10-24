BEGIN;
-- Refuse to lose references to MinIO evidence. Restore authenticated ciphertext under a reviewed operator procedure first.
ALTER TABLE supplier_documents ALTER COLUMN encrypted_content SET NOT NULL;
DROP INDEX supplier_documents_object_identity;
ALTER TABLE supplier_documents DROP CONSTRAINT supplier_documents_storage;
ALTER TABLE supplier_documents DROP COLUMN object_bucket, DROP COLUMN object_key;
COMMIT;
