BEGIN;
ALTER TABLE auth.users ADD COLUMN edit_version bigint NOT NULL DEFAULT 1;
ALTER TABLE auth.roles ADD COLUMN edit_version bigint NOT NULL DEFAULT 1;
-- Tokens issued before access-session tracking cannot be individually revoked.
-- Invalidate those once at upgrade; passwords and account status are unchanged.
UPDATE auth.users SET credential_version=credential_version+1;
UPDATE auth.refresh_tokens SET revoked_at=clock_timestamp() WHERE revoked_at IS NULL;
CREATE TABLE auth.access_sessions (
 token_hash text PRIMARY KEY, user_id uuid NOT NULL REFERENCES auth.users(id), family_id uuid NOT NULL,
 expires_at timestamptz NOT NULL, revoked_at timestamptz
);
CREATE INDEX access_sessions_family ON auth.access_sessions(family_id);
CREATE TABLE auth.login_attempts (
 identifier_hash text PRIMARY KEY, failures integer NOT NULL DEFAULT 0,
 window_started_at timestamptz NOT NULL DEFAULT clock_timestamp(), blocked_until timestamptz
);
CREATE TABLE auth.administration_changes (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, entity_type text NOT NULL, entity_id uuid NOT NULL,
 action text NOT NULL, actor_id uuid REFERENCES auth.users(id), before_snapshot jsonb, after_snapshot jsonb,
 occurred_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TRIGGER administration_changes_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON auth.administration_changes
 FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();
CREATE FUNCTION auth.audit_administration() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE before_value jsonb; after_value jsonb; subject uuid; actor uuid;
BEGIN
 actor:=NULLIF(current_setting('epcerp.auth_actor',true),'')::uuid;
 IF TG_OP<>'INSERT' THEN before_value:=to_jsonb(OLD)-ARRAY['password_hash','credential_version','temporary_password_expires_at']; END IF;
 IF TG_OP<>'DELETE' THEN after_value:=to_jsonb(NEW)-ARRAY['password_hash','credential_version','temporary_password_expires_at']; END IF;
 IF TG_TABLE_NAME IN ('users','roles') THEN
  subject:=COALESCE(after_value->>'id',before_value->>'id')::uuid;
  IF TG_OP='UPDATE' AND (before_value-ARRAY['updated_at','updated_by_id','edit_version','account_version']) IS NOT DISTINCT FROM (after_value-ARRAY['updated_at','updated_by_id','edit_version','account_version']) THEN RETURN NULL; END IF;
 ELSE subject:=COALESCE(after_value->>'user_id',before_value->>'user_id',after_value->>'role_id',before_value->>'role_id')::uuid;
 END IF;
 INSERT INTO auth.administration_changes(entity_type,entity_id,action,actor_id,before_snapshot,after_snapshot)
 VALUES(TG_TABLE_NAME,subject,lower(TG_OP),actor,before_value,after_value);
 RETURN NULL;
END $$;
CREATE FUNCTION auth.advance_edit_version() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN NEW.edit_version:=OLD.edit_version+1; RETURN NEW; END $$;
CREATE TRIGGER users_edit_version BEFORE UPDATE ON auth.users FOR EACH ROW EXECUTE FUNCTION auth.advance_edit_version();
CREATE TRIGGER roles_edit_version BEFORE UPDATE ON auth.roles FOR EACH ROW EXECUTE FUNCTION auth.advance_edit_version();
CREATE FUNCTION auth.advance_assignment_version() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF TG_TABLE_NAME='user_roles' THEN UPDATE auth.users SET edit_version=edit_version WHERE id=COALESCE(NEW.user_id,OLD.user_id);
 ELSE UPDATE auth.roles SET edit_version=edit_version WHERE id=COALESCE(NEW.role_id,OLD.role_id); END IF; RETURN NULL;
END $$;
CREATE TRIGGER user_roles_version AFTER INSERT OR DELETE ON auth.user_roles FOR EACH ROW EXECUTE FUNCTION auth.advance_assignment_version();
CREATE TRIGGER role_permissions_version AFTER INSERT OR DELETE ON auth.role_permissions FOR EACH ROW EXECUTE FUNCTION auth.advance_assignment_version();
CREATE TRIGGER users_administration_audit AFTER INSERT OR UPDATE ON auth.users FOR EACH ROW EXECUTE FUNCTION auth.audit_administration();
CREATE TRIGGER roles_administration_audit AFTER INSERT OR UPDATE ON auth.roles FOR EACH ROW EXECUTE FUNCTION auth.audit_administration();
CREATE TRIGGER user_roles_administration_audit AFTER INSERT OR DELETE ON auth.user_roles FOR EACH ROW EXECUTE FUNCTION auth.audit_administration();
CREATE TRIGGER role_permissions_administration_audit AFTER INSERT OR DELETE ON auth.role_permissions FOR EACH ROW EXECUTE FUNCTION auth.audit_administration();
CREATE TABLE auth.email_changes (
 token_hash text PRIMARY KEY, user_id uuid NOT NULL REFERENCES auth.users(id), email text NOT NULL,
 previous_email text NOT NULL, credential_version bigint NOT NULL, created_by_id uuid NOT NULL REFERENCES auth.users(id),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(), expires_at timestamptz NOT NULL,
 consumed_at timestamptz
);
CREATE UNIQUE INDEX email_changes_one_pending ON auth.email_changes(user_id) WHERE consumed_at IS NULL;
COMMIT;
