BEGIN;
ALTER TABLE catalogue_items DROP CONSTRAINT catalogue_items_category_id_serial_key;
CREATE UNIQUE INDEX catalogue_items_category_code_serial ON catalogue_items ((snapshot->'configuration'->>'category_code'),serial);
COMMIT;
