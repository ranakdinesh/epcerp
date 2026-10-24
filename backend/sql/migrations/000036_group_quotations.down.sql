BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM purchase_group_quotations) OR EXISTS(SELECT 1 FROM purchase_quotation_attachments) THEN RAISE EXCEPTION 'Cannot remove populated quotation evidence'; END IF; END $$;
DROP TABLE purchase_quotation_evidence_events,purchase_quotation_events,purchase_quotation_revisions,purchase_quotation_attachments,purchase_group_quotations;
DROP FUNCTION protect_quotation_revision_scope(),protect_quotation_owner(),protect_quotation_evidence();
COMMIT;
