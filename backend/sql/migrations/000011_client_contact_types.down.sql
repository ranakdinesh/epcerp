BEGIN;
ALTER TABLE client_contacts DROP COLUMN contact_type;
COMMIT;
