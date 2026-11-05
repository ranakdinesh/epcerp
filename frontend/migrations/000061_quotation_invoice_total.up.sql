BEGIN;
ALTER TABLE purchase_quotation_revisions ADD COLUMN invoice_total numeric(38,2)
 GENERATED ALWAYS AS ((quote->'calculation'->>'total')::numeric) STORED;
CREATE INDEX purchase_quotation_revision_total ON purchase_quotation_revisions(document_id,invoice_total);
COMMIT;
