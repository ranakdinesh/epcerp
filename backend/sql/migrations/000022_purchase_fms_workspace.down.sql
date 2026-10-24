DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM purchase_fms_workspaces) OR EXISTS(SELECT 1 FROM purchase_fms_owner_history) THEN
 RAISE EXCEPTION 'Cannot remove populated Purchase FMS workspace/history'; END IF;
END $$;
DROP TABLE purchase_fms_owner_history;
DROP FUNCTION purchase_fms_history_immutable();
DROP TABLE purchase_fms_workspaces;
