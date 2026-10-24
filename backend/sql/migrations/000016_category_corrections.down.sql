BEGIN;
-- Refuse rollback when corrections exist; do not delete immutable evidence.
ALTER TABLE category_catalogue_changes DROP CONSTRAINT category_catalogue_changes_action_check;
ALTER TABLE category_catalogue_changes ADD CONSTRAINT category_catalogue_changes_action_check CHECK(action='create');
ALTER TABLE category_catalogue_changes DROP COLUMN before_snapshot;
COMMIT;
