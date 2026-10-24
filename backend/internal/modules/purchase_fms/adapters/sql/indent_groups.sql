-- name: IndentGroupsGetGroups :one
SELECT * FROM purchase_indent_group_sets WHERE project_id= @project_id AND bo_id= @bo_id;
-- name: IndentGroupsInsertGroups :one
INSERT INTO purchase_indent_group_sets(bo_id,project_id,version,data,is_active,created_by_id,updated_by_id)VALUES(@bo_id,@project_id,1,@data,true,@actor_id,@actor_id)RETURNING *;
-- name: IndentGroupsUpdateGroups :one
UPDATE purchase_indent_group_sets SET version=version+1,data= @data,is_active= @is_active,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE bo_id= @bo_id AND project_id= @project_id AND version= @expected_version RETURNING *;
-- name: IndentGroupsSaveGroupEntry :one
INSERT INTO purchase_indent_group_entries(id,bo_id,name,scope_revision,data,is_active,created_by_id,updated_by_id)VALUES(@id,@bo_id,@name,@scope_revision,@data,true,@actor_id,@actor_id)
ON CONFLICT(id)DO UPDATE SET name=EXCLUDED.name,scope_revision=EXCLUDED.scope_revision,data=EXCLUDED.data,is_active=true,updated_at=clock_timestamp(),updated_by_id=EXCLUDED.updated_by_id WHERE purchase_indent_group_entries.bo_id=EXCLUDED.bo_id RETURNING id;
-- name: IndentGroupsDeactivateGroupEntries :exec
UPDATE purchase_indent_group_entries SET is_active=false,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE bo_id= @bo_id AND is_active AND NOT(id=ANY(@group_ids::uuid[]));
-- name: IndentGroupsInsertGroupEvent :exec
INSERT INTO purchase_indent_group_events(id,bo_id,action,reason,before_snapshot,after_snapshot,created_by_id)VALUES(@id,@bo_id,@action,@reason,@before_snapshot,@after_snapshot,@actor_id);
-- name: IndentGroupsGroupHistory :many
SELECT e.* FROM purchase_indent_group_events e JOIN purchase_indent_group_sets g ON g.bo_id=e.bo_id WHERE g.project_id= @project_id AND e.bo_id= @bo_id ORDER BY e.created_at DESC,e.id;
