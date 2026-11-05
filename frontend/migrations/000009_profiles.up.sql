BEGIN;
CREATE TABLE user_profiles (
 user_id uuid PRIMARY KEY REFERENCES users(id), display_name varchar(100) NOT NULL DEFAULT '',
 photo bytea, version bigint NOT NULL DEFAULT 1 CHECK(version > 0), is_active boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT now(), created_by_id uuid REFERENCES users(id),
 updated_at timestamptz NOT NULL DEFAULT now(), updated_by_id uuid REFERENCES users(id),
 CHECK(photo IS NULL OR octet_length(photo) <= 262144)
);
ALTER TABLE users ADD COLUMN account_version bigint NOT NULL DEFAULT 0;
COMMIT;
