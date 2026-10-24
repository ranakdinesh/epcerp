BEGIN;
DROP INDEX purchase_quotation_revision_total;
ALTER TABLE purchase_quotation_revisions DROP COLUMN invoice_total;
COMMIT;
