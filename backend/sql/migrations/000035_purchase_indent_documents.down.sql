BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM purchase_indent_documents) OR EXISTS(SELECT 1 FROM purchase_indent_document_events) OR EXISTS(SELECT 1 FROM purchase_indent_dispatch_notices) THEN RAISE EXCEPTION 'Refuse to remove populated vendor indent evidence';END IF;END $$;
DROP TABLE purchase_indent_dispatch_notices,purchase_indent_document_events,purchase_indent_documents;
DROP FUNCTION protect_indent_notice_owner(),protect_indent_document_scope(),protect_indent_document_evidence();
COMMIT;
