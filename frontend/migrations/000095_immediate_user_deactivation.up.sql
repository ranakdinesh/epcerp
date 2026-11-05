BEGIN;
SET LOCAL lock_timeout = '10s';
-- Revoking access must work before a reporting profile or handover inventory exists.
ALTER TABLE auth.auth_offboarding_cases ALTER COLUMN manager_id DROP NOT NULL;
ALTER TABLE auth.auth_offboarding_cases DROP CONSTRAINT auth_offboarding_cases_state_check;
ALTER TABLE auth.auth_offboarding_cases ADD CONSTRAINT auth_offboarding_cases_state_check CHECK(state IN ('pending','planning','approved','completed','cancelled'));
DROP INDEX auth.auth_one_open_offboarding;
CREATE UNIQUE INDEX auth_one_open_offboarding ON auth.auth_offboarding_cases(user_id) WHERE state IN ('pending','planning','approved');
CREATE TABLE auth.auth_user_status_changes (
 id uuid PRIMARY KEY, user_id uuid NOT NULL REFERENCES auth.users(id), actor_id uuid NOT NULL REFERENCES auth.users(id),
 is_active boolean NOT NULL, reason text NOT NULL CHECK(length(reason) BETWEEN 10 AND 1000),
 credential_version bigint NOT NULL, created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(user_id,credential_version)
);
CREATE TRIGGER auth_user_status_changes_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON auth.auth_user_status_changes
FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();
COMMIT;
