BEGIN;
DROP TABLE purchase_loi_document_events;
DROP TABLE purchase_loi_documents;
DROP FUNCTION check_purchase_loi_generation();
DROP FUNCTION check_purchase_loi_audit();
DROP FUNCTION check_purchase_loi_handoff();
COMMIT;
