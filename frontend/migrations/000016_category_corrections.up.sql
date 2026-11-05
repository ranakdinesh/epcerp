BEGIN;
ALTER TABLE category_catalogue_changes DROP CONSTRAINT category_catalogue_changes_action_check;
ALTER TABLE category_catalogue_changes ADD CONSTRAINT category_catalogue_changes_action_check CHECK(action IN ('create','update'));
ALTER TABLE category_catalogue_changes ADD COLUMN before_snapshot jsonb NOT NULL DEFAULT '{}';
COMMIT;
