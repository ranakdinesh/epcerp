BEGIN;
ALTER TABLE supplier_documents ALTER COLUMN encrypted_content DROP NOT NULL;
ALTER TABLE supplier_documents ADD COLUMN object_bucket text, ADD COLUMN object_key text;
ALTER TABLE supplier_documents ADD CONSTRAINT supplier_documents_storage CHECK (
 (encrypted_content IS NOT NULL AND object_bucket IS NULL AND object_key IS NULL) OR
 (encrypted_content IS NULL AND length(object_bucket)>0 AND length(object_key)>0 AND object_bucket IS NOT NULL AND object_key IS NOT NULL)
);
CREATE UNIQUE INDEX supplier_documents_object_identity ON supplier_documents(object_bucket,object_key) WHERE object_key IS NOT NULL;
COMMIT;
