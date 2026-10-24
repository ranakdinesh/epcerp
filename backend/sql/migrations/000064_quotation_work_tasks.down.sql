BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM purchase_quotation_work_tasks) THEN RAISE EXCEPTION 'Retain quotation tasks';END IF;END $$;
DROP TABLE purchase_quotation_work_tasks;
COMMIT;
