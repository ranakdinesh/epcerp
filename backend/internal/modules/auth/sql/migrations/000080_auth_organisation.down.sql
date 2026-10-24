DROP TABLE auth_organisation_changes;
DROP FUNCTION reject_auth_organisation_change_mutation();
DROP TABLE auth_reporting_lines;
DROP FUNCTION guard_auth_reporting_line();
DROP TABLE auth_departments;
ALTER TABLE users DROP COLUMN mobile, DROP COLUMN last_name, DROP COLUMN first_name;
