CREATE TABLE auth_password_recovery (
    token_hash text PRIMARY KEY CHECK (length(token_hash) = 64),
    user_id uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    CHECK (expires_at > created_at)
);
CREATE INDEX auth_password_recovery_user_created ON auth_password_recovery(user_id, created_at DESC);
