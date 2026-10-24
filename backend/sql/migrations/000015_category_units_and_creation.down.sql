BEGIN;
DROP TABLE category_catalogue_changes;
ALTER TABLE category_subcategories DROP CONSTRAINT subcategory_inherits_unit;
ALTER TABLE categories DROP CONSTRAINT categories_id_unit_unique;
ALTER TABLE categories DROP COLUMN unit_id;
COMMIT;
