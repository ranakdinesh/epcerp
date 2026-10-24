BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM purchase_orders) OR EXISTS(SELECT 1 FROM purchase_order_sequences) THEN RAISE EXCEPTION 'Cannot rollback populated purchase orders';END IF;END $$;
DROP TABLE purchase_order_document_events,purchase_order_events,purchase_orders,purchase_order_sequences;
DROP FUNCTION protect_purchase_order(),protect_purchase_order_history(),check_purchase_order_write(),check_purchase_order_event(),check_purchase_order_document();
COMMIT;
