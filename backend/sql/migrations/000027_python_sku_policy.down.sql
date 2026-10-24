BEGIN;
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM catalogue_items GROUP BY category_id,serial HAVING count(*)>1) THEN
  RAISE EXCEPTION 'Category-code numbering cannot revert: issued numbers overlap within a category identity';
 END IF;
END $$;
DROP INDEX catalogue_items_category_code_serial;
ALTER TABLE catalogue_items ADD CONSTRAINT catalogue_items_category_id_serial_key UNIQUE(category_id,serial);
COMMIT;
