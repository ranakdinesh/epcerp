BEGIN;
ALTER TABLE purchase_technical_files ALTER COLUMN ciphertext DROP NOT NULL;
ALTER TABLE purchase_technical_files ADD COLUMN object_bucket text NOT NULL DEFAULT '', ADD COLUMN object_key text NOT NULL DEFAULT '';
ALTER TABLE purchase_technical_files ADD CONSTRAINT technical_file_storage_location CHECK((object_bucket='' AND object_key='' AND ciphertext IS NOT NULL) OR (object_bucket<>'' AND object_key<>'' AND ciphertext IS NULL));
CREATE UNIQUE INDEX technical_object_key ON purchase_technical_files(object_bucket,object_key) WHERE object_key<>'';
COMMIT;
