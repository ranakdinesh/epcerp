DO $$ BEGIN IF EXISTS(SELECT 1 FROM supplier_document_approvals) OR EXISTS(SELECT 1 FROM supplier_document_tasks) THEN RAISE EXCEPTION 'Document approval/task evidence must be preserved';END IF;END $$;
DROP TABLE supplier_document_approval_changes;
DROP TABLE supplier_document_tasks;
DROP TABLE supplier_document_approvals;
DROP FUNCTION epcerp_preserve_completed_document_task();
