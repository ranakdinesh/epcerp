-- name: AdvancesGetAdvance :one
SELECT * FROM purchase_advances WHERE project_id= @project_id AND package_id= @package_id FOR UPDATE;
-- name: AdvancesInsertAdvance :one
INSERT INTO purchase_advances(id,project_id,package_id,order_id,order_version,version,state,source_snapshot,data,submitted_snapshot,source_fingerprint,decision_id,is_active,created_at,updated_at,created_by_id,updated_by_id)
VALUES(@id,@project_id,@package_id,@order_id,@order_version,1,@state,@source_snapshot,@data,@submitted_snapshot,@source_fingerprint,@decision_id,true,@at,@at,@actor_id,@actor_id) RETURNING *;
-- name: AdvancesUpdateAdvance :one
UPDATE purchase_advances SET version=version+1,state= @state,source_snapshot= @source_snapshot,data= @data,submitted_snapshot= @submitted_snapshot,source_fingerprint= @source_fingerprint,decision_id= @decision_id,updated_at= @at,updated_by_id= @actor_id
WHERE id= @id AND version= @expected_version AND is_active RETURNING *;
-- name: AdvancesEvent :exec
INSERT INTO purchase_advance_events(id,subject_id,action,reason,before_snapshot,after_snapshot,created_at,created_by_id)
VALUES(@id,@request_id,@action,@reason,@before_snapshot,@after_snapshot,@at,@actor_id);
-- name: AdvancesHistory :many
SELECT * FROM purchase_advance_events WHERE subject_id=$1 ORDER BY created_at DESC,id DESC;
-- name: AdvancesQueueRecords :many
SELECT * FROM purchase_advances WHERE state<>'draft' AND is_active ORDER BY updated_at DESC,id DESC LIMIT 50 OFFSET $1;
