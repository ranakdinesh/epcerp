-- name: AwardsLock :exec
SELECT pg_advisory_xact_lock(hashtextextended('purchase-award:'|| @project::text,0));
-- name: AwardsGet :one
SELECT * FROM purchase_award_handoffs WHERE project_id= @project_id AND attempt_id= @attempt_id;
-- name: AwardsInsert :exec
INSERT INTO purchase_award_handoffs(id,project_id,bo_id,attempt_id,packages,reason,is_active,created_at,updated_at,created_by_id,updated_by_id)
VALUES(@id,@project_id,@bo_id,@attempt_id,@packages,@reason,true,@at,@at,@actor_id,@actor_id);
