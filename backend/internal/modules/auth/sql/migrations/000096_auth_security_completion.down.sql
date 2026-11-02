BEGIN;
DROP TRIGGER role_permissions_administration_audit ON auth.role_permissions;
DROP TRIGGER user_roles_administration_audit ON auth.user_roles;
DROP TRIGGER roles_administration_audit ON auth.roles;
DROP TRIGGER users_administration_audit ON auth.users;
DROP TRIGGER user_roles_version ON auth.user_roles;
DROP TRIGGER role_permissions_version ON auth.role_permissions;
DROP TRIGGER users_edit_version ON auth.users;
DROP TRIGGER roles_edit_version ON auth.roles;
DROP FUNCTION auth.advance_assignment_version();
DROP FUNCTION auth.advance_edit_version();
DROP FUNCTION auth.audit_administration();
-- Preserve security history: downgrade is rejected after any audited use.
DO $$ BEGIN IF EXISTS(SELECT 1 FROM auth.administration_changes) THEN RAISE EXCEPTION 'Security audit history exists; downgrade requires a forward migration'; END IF; END $$;
DROP TABLE auth.email_changes,auth.administration_changes,auth.login_attempts,auth.access_sessions;
ALTER TABLE auth.users DROP COLUMN edit_version;
ALTER TABLE auth.roles DROP COLUMN edit_version;
COMMIT;
