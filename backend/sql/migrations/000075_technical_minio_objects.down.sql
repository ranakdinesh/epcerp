BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM purchase_technical_files WHERE object_key<>'') THEN RAISE EXCEPTION 'Retain private technical objects and metadata';END IF;END $$;
DROP INDEX technical_object_key;
ALTER TABLE purchase_technical_files DROP CONSTRAINT technical_file_storage_location;
ALTER TABLE purchase_technical_files DROP COLUMN object_bucket,DROP COLUMN object_key;
ALTER TABLE purchase_technical_files ALTER COLUMN ciphertext SET NOT NULL;
COMMIT;
