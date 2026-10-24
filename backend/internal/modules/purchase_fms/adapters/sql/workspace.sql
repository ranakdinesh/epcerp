-- name: WorkspaceGetWorkspace :one
SELECT * FROM purchase_fms_workspaces WHERE project_id=$1;
-- name: WorkspaceAssignedProjects :many
SELECT project_id FROM purchase_fms_workspaces WHERE purchase_manager_id=$1 AND is_active ORDER BY project_id;
-- name: WorkspaceInsertWorkspace :one
INSERT INTO purchase_fms_workspaces(project_id,purchase_manager_id,created_by_id,updated_by_id) VALUES(sqlc.arg(project_id),sqlc.arg(manager_id),sqlc.arg(actor_id),sqlc.arg(actor_id)) RETURNING *;
-- name: WorkspaceTransferOwner :one
UPDATE purchase_fms_workspaces SET purchase_manager_id=sqlc.arg(manager_id),updated_by_id=sqlc.arg(actor_id),updated_at=clock_timestamp(),version=version+1 WHERE project_id=sqlc.arg(project_id) AND version=sqlc.arg(expected_version) AND is_active RETURNING *;
-- name: WorkspaceInsertOwnerHistory :one
INSERT INTO purchase_fms_owner_history(id,project_id,previous_manager_id,purchase_manager_id,workspace_version,reason,created_by_id)VALUES(sqlc.arg(id),sqlc.arg(project_id),sqlc.narg(previous_manager_id),sqlc.arg(manager_id),sqlc.arg(version),sqlc.arg(reason),sqlc.arg(actor_id)) RETURNING *;
-- name: WorkspaceOwnerHistory :many
SELECT * FROM purchase_fms_owner_history WHERE project_id=$1 ORDER BY workspace_version DESC;
