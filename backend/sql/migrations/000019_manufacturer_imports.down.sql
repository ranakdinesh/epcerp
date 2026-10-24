BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM manufacturer_imports) THEN RAISE EXCEPTION 'Cannot remove populated manufacturer import evidence'; END IF; END $$;
DROP TRIGGER immutable_manufacturer_event ON manufacturers_events;
DROP TABLE manufacturer_imports;
DROP FUNCTION protect_manufacturer_evidence();
COMMIT;
