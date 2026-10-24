ALTER TABLE users ADD COLUMN must_change_password boolean NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN temporary_password_expires_at timestamptz;
ALTER TABLE users ADD COLUMN credential_version bigint NOT NULL DEFAULT 0 CHECK (credential_version >= 0);
CREATE TABLE auth_credential_emails (
 id uuid PRIMARY KEY,
 user_id uuid NOT NULL REFERENCES users(id),
 credential_version bigint NOT NULL,
 payload bytea,
 payload_hash text NOT NULL,
 state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','sending','delivered','failed','cancelled')),
 attempts integer NOT NULL DEFAULT 0,
 lease_id uuid,
 lease_until timestamptz,
 next_attempt_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 created_by_id uuid REFERENCES users(id),
 updated_by_id uuid REFERENCES users(id)
);
CREATE INDEX auth_credential_emails_pending ON auth_credential_emails(next_attempt_at) WHERE state IN ('pending','sending');
CREATE INDEX auth_credential_emails_user ON auth_credential_emails(user_id,created_at DESC);
CREATE TABLE auth_credential_changes (
 id uuid PRIMARY KEY,
 user_id uuid NOT NULL REFERENCES users(id),
 actor_id uuid NOT NULL REFERENCES users(id),
 action text NOT NULL,
 reason text NOT NULL CHECK (length(reason) BETWEEN 10 AND 1000),
 credential_version bigint NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE FUNCTION reject_auth_credential_change_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'Credential history is immutable'; END; $$;
CREATE TRIGGER auth_credential_changes_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON auth_credential_changes
FOR EACH STATEMENT EXECUTE FUNCTION reject_auth_credential_change_mutation();
