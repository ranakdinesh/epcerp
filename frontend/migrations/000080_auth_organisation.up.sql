ALTER TABLE users ADD COLUMN first_name text NOT NULL DEFAULT '' CHECK (length(first_name)<=100);
ALTER TABLE users ADD COLUMN last_name text NOT NULL DEFAULT '' CHECK (length(last_name)<=100);
ALTER TABLE users ADD COLUMN mobile text NOT NULL DEFAULT '' CHECK (length(mobile)<=24);
CREATE TABLE auth_departments (
 id uuid PRIMARY KEY, code text NOT NULL UNIQUE CHECK (length(code) BETWEEN 2 AND 32),
 name text NOT NULL CHECK (length(name) BETWEEN 2 AND 150), manager_id uuid REFERENCES users(id),
 is_active boolean NOT NULL DEFAULT true, version bigint NOT NULL DEFAULT 1 CHECK(version>0),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 created_by_id uuid REFERENCES users(id), updated_by_id uuid REFERENCES users(id)
);
CREATE TABLE auth_reporting_lines (
 user_id uuid PRIMARY KEY REFERENCES users(id), department_id uuid NOT NULL REFERENCES auth_departments(id),
 manager_id uuid REFERENCES users(id), is_company_root boolean NOT NULL DEFAULT false,
 is_active boolean NOT NULL DEFAULT true, version bigint NOT NULL DEFAULT 1 CHECK(version>0),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 created_by_id uuid REFERENCES users(id), updated_by_id uuid REFERENCES users(id),
 CHECK(user_id IS DISTINCT FROM manager_id),
 CHECK((manager_id IS NOT NULL AND NOT is_company_root) OR (manager_id IS NULL AND is_company_root))
);
CREATE INDEX auth_reporting_manager ON auth_reporting_lines(manager_id) WHERE is_active;
CREATE INDEX auth_reporting_department ON auth_reporting_lines(department_id) WHERE is_active;
CREATE TABLE auth_organisation_changes (
 id uuid PRIMARY KEY, entity_id uuid NOT NULL, entity_type text NOT NULL CHECK(entity_type IN ('employee','department')),
 version bigint NOT NULL, reason text NOT NULL CHECK(length(reason) BETWEEN 10 AND 1000),
 snapshot jsonb NOT NULL, actor_id uuid NOT NULL REFERENCES users(id), created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(entity_id,entity_type,version)
);
CREATE FUNCTION reject_auth_organisation_change_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'Organisation history is immutable'; END; $$;
CREATE TRIGGER auth_organisation_changes_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON auth_organisation_changes
FOR EACH STATEMENT EXECUTE FUNCTION reject_auth_organisation_change_mutation();
-- The application serializes organisation writes before reading the reporting graph.
-- The trigger is a second boundary for accidental direct writes; the lock also
-- serializes concurrent opposite-edge changes at PostgreSQL READ COMMITTED.
CREATE FUNCTION guard_auth_reporting_line() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(hashtextextended('auth.administration',0));
 IF NEW.is_active AND EXISTS (
  WITH RECURSIVE chain(id) AS (
   SELECT NEW.manager_id WHERE NEW.manager_id IS NOT NULL
   UNION SELECT r.manager_id FROM auth_reporting_lines r JOIN chain c ON r.user_id=c.id WHERE r.is_active AND r.manager_id IS NOT NULL
  ) SELECT 1 FROM chain WHERE id=NEW.user_id
 ) THEN RAISE EXCEPTION 'Circular reporting line' USING ERRCODE='23514'; END IF;
 RETURN NEW;
END; $$;
CREATE TRIGGER auth_reporting_line_guard BEFORE INSERT OR UPDATE ON auth_reporting_lines FOR EACH ROW EXECUTE FUNCTION guard_auth_reporting_line();
