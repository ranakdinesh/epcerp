-- name: ListDemands :many
SELECT id,project_id,item_id,engineering_key,item_snapshot,quantity::text AS quantity,required_date::text AS required_date,allocations,alternatives,is_active,version,created_at,updated_at,created_by_id,updated_by_id FROM project_material_requirements WHERE project_id= @project_id ORDER BY created_at,id;
-- name: GetDemand :one
SELECT id,project_id,item_id,engineering_key,item_snapshot,quantity::text AS quantity,required_date::text AS required_date,allocations,alternatives,is_active,version,created_at,updated_at,created_by_id,updated_by_id FROM project_material_requirements WHERE project_id= @project_id AND id= @id;
-- name: InsertDemand :one
INSERT INTO project_material_requirements(id,project_id,item_id,engineering_key,item_snapshot,quantity,required_date,allocations,alternatives,is_active,version,created_by_id,updated_by_id)VALUES(@id,@project_id,@item_id,@engineering_key,@item_snapshot,(@quantity::text)::numeric,(@required_date::text)::date,@allocations,@alternatives,@is_active,1,@actor_id,@actor_id) RETURNING id,project_id,item_id,engineering_key,item_snapshot,quantity::text AS quantity,required_date::text AS required_date,allocations,alternatives,is_active,version,created_at,updated_at,created_by_id,updated_by_id;
-- name: UpdateDemand :one
UPDATE project_material_requirements SET quantity=(@quantity::text)::numeric,required_date=(@required_date::text)::date,allocations= @allocations,alternatives= @alternatives,is_active= @is_active,version=version+1,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE project_id= @project_id AND id= @id AND version= @expected_version RETURNING id,project_id,item_id,engineering_key,item_snapshot,quantity::text AS quantity,required_date::text AS required_date,allocations,alternatives,is_active,version,created_at,updated_at,created_by_id,updated_by_id;
-- name: InsertDemandEvent :exec
INSERT INTO project_material_events(id,subject_id,action,reason,before_snapshot,after_snapshot,created_by_id)VALUES(@id,@subject_id,@action,@reason,@before_snapshot,@after_snapshot,@actor_id);
-- name: DemandHistory :many
SELECT e.* FROM project_material_events e JOIN project_material_requirements d ON d.id=e.subject_id WHERE d.project_id= @project_id AND e.subject_id= @id ORDER BY e.created_at DESC,e.id;
