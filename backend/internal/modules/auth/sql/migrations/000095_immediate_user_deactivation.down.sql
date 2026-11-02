BEGIN;
-- An empty development database can roll back. Never erase status evidence or
-- invent reporting managers to make retained pending cases fit the old model.
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM auth.auth_user_status_changes) OR EXISTS(SELECT 1 FROM auth.auth_offboarding_cases WHERE state='pending' OR manager_id IS NULL) THEN
  RAISE EXCEPTION 'Cannot downgrade: retain user status evidence and pending handovers';
 END IF;
END $$;
DROP TABLE auth.auth_user_status_changes;
DROP INDEX auth.auth_one_open_offboarding;
CREATE UNIQUE INDEX auth_one_open_offboarding ON auth.auth_offboarding_cases(user_id) WHERE state IN ('planning','approved');
ALTER TABLE auth.auth_offboarding_cases DROP CONSTRAINT auth_offboarding_cases_state_check;
ALTER TABLE auth.auth_offboarding_cases ADD CONSTRAINT auth_offboarding_cases_state_check CHECK(state IN ('planning','approved','completed','cancelled'));
ALTER TABLE auth.auth_offboarding_cases ALTER COLUMN manager_id SET NOT NULL;
COMMIT;
