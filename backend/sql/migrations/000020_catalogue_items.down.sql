BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM catalogue_items) OR EXISTS(SELECT 1 FROM catalogue_item_events) THEN RAISE EXCEPTION 'Cannot remove issued items or item history'; END IF; END $$;
DROP TABLE catalogue_item_events;
DROP TABLE catalogue_items;
DROP FUNCTION protect_catalogue_item_identity();
DROP FUNCTION protect_catalogue_item_evidence();
COMMIT;
