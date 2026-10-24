-- name: BOListBO :many
SELECT * FROM purchase_bo_lists WHERE project_id= @project_id ORDER BY created_at,id;
-- name: BOGetBO :one
SELECT * FROM purchase_bo_lists WHERE project_id= @project_id AND id= @id;
-- name: BOBOLines :many
SELECT id,bo_id,item_id,coalesce(quantity::text,'')::text AS quantity,coalesce(required_date::text,'')::text AS required_date,reference,notes,item_snapshot,demand_id FROM purchase_bo_lines WHERE bo_id= @bo_id AND is_active ORDER BY sort_order,id;
-- name: BOInsertBO :one
INSERT INTO purchase_bo_lists(id,project_id,preparer_id,name,state,is_active,version,completed_at,completed_by_id,created_by_id,updated_by_id) VALUES(@id,@project_id,@preparer_id,@name,@state,true,1,@completed_at,@completed_by_id,@actor_id,@actor_id) RETURNING *;
-- name: BOUpdateBO :one
UPDATE purchase_bo_lists SET name= @name,state= @state,version=version+1,completed_at= @completed_at,completed_by_id= @completed_by_id,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE project_id= @project_id AND id= @id AND version= @expected_version RETURNING *;
-- name: BOSaveBOLine :one
INSERT INTO purchase_bo_lines(id,bo_id,item_id,quantity,required_date,reference,notes,item_snapshot,demand_id,sort_order,is_active,created_by_id,updated_by_id)
VALUES(@id,@bo_id,@item_id,(sqlc.narg(quantity)::text)::numeric,(sqlc.narg(required_date)::text)::date,@reference,@notes,@item_snapshot,@demand_id,@sort_order,true,@actor_id,@actor_id)
ON CONFLICT(id) DO UPDATE SET item_id=EXCLUDED.item_id,quantity=EXCLUDED.quantity,required_date=EXCLUDED.required_date,reference=EXCLUDED.reference,notes=EXCLUDED.notes,item_snapshot=EXCLUDED.item_snapshot,demand_id=EXCLUDED.demand_id,sort_order=EXCLUDED.sort_order,is_active=true,updated_at=clock_timestamp(),updated_by_id=EXCLUDED.updated_by_id WHERE purchase_bo_lines.bo_id=EXCLUDED.bo_id RETURNING id;
-- name: BODeactivateMissingLines :exec
UPDATE purchase_bo_lines SET is_active=false,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE bo_id= @bo_id AND is_active AND NOT(id=ANY(@line_ids::uuid[]));
-- name: BOInsertBOEvent :exec
INSERT INTO purchase_bo_events(id,subject_id,action,reason,before_snapshot,after_snapshot,created_by_id)VALUES(@id,@subject_id,@action,@reason,@before_snapshot,@after_snapshot,@actor_id);
-- name: BOBOHistory :many
SELECT e.* FROM purchase_bo_events e JOIN purchase_bo_lists b ON b.id=e.subject_id WHERE b.project_id= @project_id AND e.subject_id= @id ORDER BY e.created_at DESC,e.id;

-- name: BOExactDemandLink :many
SELECT b.id AS bo_id,b.version AS bo_version,l.id AS line_id,l.demand_id,l.item_id,l.quantity::text AS quantity
FROM purchase_bo_lists b JOIN purchase_bo_lines l ON l.bo_id=b.id
WHERE b.project_id= @project_id AND b.is_active AND b.state='completed' AND l.is_active AND l.demand_id= @demand_id
ORDER BY b.id,l.id LIMIT 2;
