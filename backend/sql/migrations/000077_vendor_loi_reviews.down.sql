BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM purchase_loi_reviews) THEN RAISE EXCEPTION 'Cannot remove populated LOI review history'; END IF; END $$;
DROP TABLE purchase_loi_reviews;
DROP FUNCTION check_purchase_loi_review_audit();
COMMIT;
