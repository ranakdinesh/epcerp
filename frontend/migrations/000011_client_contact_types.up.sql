BEGIN;
-- Existing contacts remain unclassified until reviewed; do not infer from designation.
ALTER TABLE client_contacts ADD COLUMN contact_type VARCHAR(16) NOT NULL DEFAULT '';
ALTER TABLE client_contacts ADD CONSTRAINT client_contacts_type_check
 CHECK (contact_type IN ('', 'EIC', 'HOD', 'SITE_CONTACT'));
COMMIT;
