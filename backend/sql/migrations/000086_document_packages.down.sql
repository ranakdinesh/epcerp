BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM document_package_receipts) THEN RAISE EXCEPTION 'Cannot discard retained document package evidence'; END IF;END $$;
DROP TABLE document_package_operations,document_package_events,document_package_members,document_package_objects,document_package_files,document_package_receipts;
DROP FUNCTION protect_package_file(); DROP FUNCTION protect_consumed_package();
COMMIT;
