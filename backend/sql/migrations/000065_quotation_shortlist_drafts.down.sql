BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM purchase_quotation_shortlist_history) THEN RAISE EXCEPTION 'Cannot remove recorded shortlist history';END IF;END $$;
DROP TABLE purchase_quotation_shortlist_history;
DROP TABLE purchase_quotation_shortlist_drafts;
COMMIT;
