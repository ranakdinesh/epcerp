CREATE TABLE auth_offboarding_cases (
 id uuid PRIMARY KEY, user_id uuid NOT NULL REFERENCES users(id), manager_id uuid NOT NULL REFERENCES users(id),
 profile_version bigint NOT NULL, state text NOT NULL CHECK(state IN ('planning','approved','completed','cancelled')),
 reason text NOT NULL CHECK(length(reason) BETWEEN 10 AND 1000), items jsonb NOT NULL CHECK(jsonb_typeof(items)='array'),
 approval_by_id uuid REFERENCES users(id), approved_at timestamptz,
 is_active boolean NOT NULL DEFAULT true, version bigint NOT NULL DEFAULT 1 CHECK(version>0),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 created_by_id uuid REFERENCES users(id), updated_by_id uuid REFERENCES users(id),
 CHECK(user_id<>manager_id),CHECK(state<>'approved' OR (approval_by_id IS NOT NULL AND approval_by_id=manager_id AND approved_at IS NOT NULL))
);
CREATE UNIQUE INDEX auth_one_open_offboarding ON auth_offboarding_cases(user_id) WHERE state IN ('planning','approved');
CREATE INDEX auth_manager_offboarding ON auth_offboarding_cases(manager_id,state);
CREATE TABLE auth_offboarding_changes (
 id uuid PRIMARY KEY, case_id uuid NOT NULL REFERENCES auth_offboarding_cases(id), version bigint NOT NULL,
 action text NOT NULL, reason text NOT NULL CHECK(length(reason) BETWEEN 10 AND 1000), snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(), created_by_id uuid NOT NULL REFERENCES users(id),UNIQUE(case_id,version)
);
CREATE TRIGGER auth_offboarding_changes_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON auth_offboarding_changes
FOR EACH STATEMENT EXECUTE FUNCTION reject_auth_organisation_change_mutation();
