BEGIN;
ALTER TABLE users DROP COLUMN account_version;
DROP TABLE user_profiles;
COMMIT;
