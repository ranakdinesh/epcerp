BEGIN;
ALTER TABLE categories ADD COLUMN unit_id uuid REFERENCES catalogue_units(id);
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM categories c LEFT JOIN category_subcategories s ON s.category_id=c.id GROUP BY c.id HAVING count(DISTINCT s.unit_id) <> 1) THEN
  RAISE EXCEPTION 'Category base units are missing or ambiguous; review before migration';
 END IF;
END $$;
UPDATE categories c SET unit_id=(SELECT s.unit_id FROM category_subcategories s WHERE s.category_id=c.id LIMIT 1);
ALTER TABLE categories ALTER COLUMN unit_id SET NOT NULL;
ALTER TABLE categories ADD CONSTRAINT categories_id_unit_unique UNIQUE(id,unit_id);
ALTER TABLE category_subcategories ADD CONSTRAINT subcategory_inherits_unit FOREIGN KEY(category_id,unit_id) REFERENCES categories(id,unit_id);
CREATE TABLE category_catalogue_changes (
 id uuid PRIMARY KEY, kind varchar(16) NOT NULL CHECK(kind IN ('category','subcategory','unit')),
 subject_id uuid NOT NULL, action varchar(16) NOT NULL CHECK(action='create'),
 reason varchar(2000) NOT NULL, after_snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), created_by_id uuid REFERENCES users(id)
);
CREATE TRIGGER category_catalogue_changes_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON category_catalogue_changes FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
COMMIT;
