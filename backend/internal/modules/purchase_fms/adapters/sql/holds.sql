-- name: HoldsListHolds :many
SELECT * FROM purchase_holds WHERE project_id= @project_id ORDER BY created_at,id;
-- name: HoldsGetHold :one
SELECT * FROM purchase_holds WHERE project_id= @project_id AND id= @id;
-- name: HoldsInsertHold :one
INSERT INTO purchase_holds(id,project_id,doer_id,resolver_id,data,is_active,version,created_by_id,updated_by_id)VALUES(@id,@project_id,@doer_id,@resolver_id,@data,true,1,@actor_id,@actor_id) RETURNING *;
-- name: HoldsUpdateHold :one
UPDATE purchase_holds SET data= @data,is_active= @is_active,version=version+1,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE project_id= @project_id AND id= @id AND version= @expected_version RETURNING *;
-- name: HoldsInsertHoldEvent :exec
INSERT INTO purchase_hold_events(id,subject_id,action,reason,before_snapshot,after_snapshot,created_by_id)VALUES(@id,@subject_id,@action,@reason,@before_snapshot,@after_snapshot,@actor_id);
-- name: HoldsHoldHistory :many
SELECT e.* FROM purchase_hold_events e JOIN purchase_holds h ON h.id=e.subject_id WHERE h.project_id= @project_id AND e.subject_id= @id ORDER BY e.created_at DESC,e.id;
