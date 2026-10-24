-- name: IndentsListIndents :many
SELECT * FROM purchase_indents WHERE project_id= @project_id ORDER BY created_at,id;
-- name: IndentsGetIndent :one
SELECT * FROM purchase_indents WHERE project_id= @project_id AND id= @id;
-- name: IndentsInsertIndent :one
INSERT INTO purchase_indents(id,project_id,doer_id,resolver_id,data,is_active,version,created_by_id,updated_by_id)VALUES(@id,@project_id,@doer_id,@resolver_id,@data,true,1,@actor_id,@actor_id) RETURNING *;
-- name: IndentsUpdateIndent :one
UPDATE purchase_indents SET doer_id= @doer_id,resolver_id= @resolver_id,data= @data,is_active= @is_active,version=version+1,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE project_id= @project_id AND id= @id AND version= @expected_version RETURNING *;
-- name: IndentsInsertIndentEvent :exec
INSERT INTO purchase_indent_events(id,subject_id,action,reason,before_snapshot,after_snapshot,created_by_id)VALUES(@id,@subject_id,@action,@reason,@before_snapshot,@after_snapshot,@actor_id);
-- name: IndentsIndentHistory :many
SELECT e.* FROM purchase_indent_events e JOIN purchase_indents h ON h.id=e.subject_id WHERE h.project_id= @project_id AND e.subject_id= @id ORDER BY e.created_at DESC,e.id;

-- name: IndentsInsertRelease :execrows
INSERT INTO purchase_indent_releases(indent_id,demand_id,quantity,snapshot,created_by_id)VALUES(@indent_id,@demand_id,(@quantity::text)::numeric,@snapshot,@actor_id)ON CONFLICT DO NOTHING;
-- name: IndentsGetRelease :one
SELECT snapshot FROM purchase_indent_releases WHERE indent_id= @indent_id AND demand_id= @demand_id;
-- name: IndentsCommitted :one
SELECT coalesce(sum(r.quantity),0)::text AS quantity FROM purchase_indent_releases r JOIN purchase_indents i ON i.id=r.indent_id WHERE i.project_id= @project_id AND r.demand_id= @demand_id;
