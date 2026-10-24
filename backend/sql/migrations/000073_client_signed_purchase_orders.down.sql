BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM purchase_order_client_files) OR EXISTS(SELECT 1 FROM purchase_order_events WHERE action IN('submit_client','client_decision')) THEN RAISE EXCEPTION 'Retain recorded client PO evidence; rollback is unsafe';END IF;END $$;
DROP TRIGGER po_client_signoff_guard ON purchase_orders;
DROP FUNCTION check_po_client_signoff();
DROP TABLE purchase_order_client_file_events,purchase_order_client_files;
DROP FUNCTION check_po_client_file_event(),check_po_client_file_write();
ALTER TABLE purchase_order_events DROP CONSTRAINT purchase_order_events_action_check;
ALTER TABLE purchase_order_events ADD CONSTRAINT purchase_order_events_action_check CHECK(action IN('prepare','release'));
COMMIT;
