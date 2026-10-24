DROP TABLE auth_credential_changes;
DROP FUNCTION reject_auth_credential_change_mutation();
DROP TABLE auth_credential_emails;
ALTER TABLE users DROP COLUMN credential_version;
ALTER TABLE users DROP COLUMN temporary_password_expires_at;
ALTER TABLE users DROP COLUMN must_change_password;
