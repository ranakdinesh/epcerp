CREATE TABLE auth_username_changes (
 id uuid PRIMARY KEY,user_id uuid NOT NULL REFERENCES users(id),version bigint NOT NULL CHECK(version>0),
 previous_username text NOT NULL,username text NOT NULL,created_by_id uuid NOT NULL REFERENCES users(id),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),UNIQUE(user_id,version),
 CHECK(created_by_id=user_id),CHECK(previous_username<>username)
);
CREATE TRIGGER auth_username_changes_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON auth_username_changes
FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
-- Technical singleton: bind the recovery administrator to its immutable user ID.
CREATE TABLE auth_bootstrap_identity (
 singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),user_id uuid NOT NULL UNIQUE REFERENCES users(id),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TRIGGER auth_bootstrap_identity_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON auth_bootstrap_identity
FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
