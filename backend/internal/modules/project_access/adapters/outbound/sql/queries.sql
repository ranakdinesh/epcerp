-- name: ListProfiles :many
SELECT * FROM project_responsibility_profiles WHERE project_id= @project_id ORDER BY bundle_id,id LIMIT 1001;
-- name: GetProfile :one
SELECT * FROM project_responsibility_profiles WHERE project_id= @project_id AND id= @id;
-- name: InsertProfile :one
INSERT INTO project_responsibility_profiles(id,project_id,user_id,bundle_id,responsibility,responsibility_key,permission_codes,is_active,version,created_by_id,updated_by_id)
VALUES(@id,@project_id,@user_id,@bundle_id,@responsibility,@responsibility_key,@permission_codes,true,1,@actor_id,@actor_id) RETURNING *;
-- name: UpdateProfile :one
UPDATE project_responsibility_profiles SET responsibility= @responsibility,responsibility_key= @responsibility_key,permission_codes= @permission_codes,is_active= @is_active,version=version+1,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE project_id= @project_id AND id= @id AND version= @expected_version RETURNING *;
-- name: InsertProfileEvent :exec
INSERT INTO project_responsibility_events(id,profile_id,project_id,action,reason,before_snapshot,after_snapshot,created_by_id)VALUES(@id,@profile_id,@project_id,@action,@reason,@before_snapshot,@after_snapshot,@actor_id);
-- name: ProfileHistory :many
SELECT * FROM project_responsibility_events WHERE project_id= @project_id AND profile_id= @profile_id ORDER BY created_at DESC,id DESC LIMIT 200;
-- name: AssignedProjectCandidates :many
SELECT DISTINCT project_id FROM project_responsibility_profiles WHERE user_id= @user_id AND is_active ORDER BY project_id LIMIT 1001;

-- name: ReadWorkInventory :many
WITH assignments AS (
SELECT 'project_responsibility'::text AS kind,a.id::text AS id,a.user_id::text AS owner_id,a.version::bigint AS version,a.responsibility::text AS title,'active'::text AS state,'/dashboard?screen=projects&project='||a.project_id::text::text AS link,a.permission_codes::jsonb AS required_permissions,''::text AS due_date, a.project_id::text AS project_id FROM project_responsibility_profiles a WHERE a.is_active
) SELECT jsonb_build_object('Source','project_access','Kind',kind,'ID',id,'OwnerID',owner_id,'Version',version,'Title',title,'State',state,'Link',link,'RequiredPermissions',required_permissions,'DueDate',due_date,'ProjectID',project_id)::jsonb AS payload FROM assignments WHERE (sqlc.arg(owner_id)::text='' OR owner_id=sqlc.arg(owner_id)) AND (sqlc.arg(record_id)::text='' OR id=sqlc.arg(record_id)) AND (sqlc.arg(kind)::text='' OR kind=sqlc.arg(kind)) ORDER BY kind,id,owner_id LIMIT 10001;
