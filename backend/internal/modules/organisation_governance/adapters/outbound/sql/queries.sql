-- name: LockDirectorSlot :exec
SELECT pg_advisory_xact_lock(hashtext('epcerp.organisation.director'));
-- name: GetDirector :one
SELECT * FROM organisation_director_designation WHERE slot=true;
-- name: CreateDirector :one
INSERT INTO organisation_director_designation(id,user_id,version,is_active,created_by_id,updated_by_id)VALUES($1,$2,1,$3,$4,$4)RETURNING *;
-- name: UpdateDirector :one
UPDATE organisation_director_designation SET user_id=sqlc.arg(user_id),version=version+1,is_active=sqlc.arg(is_active),updated_by_id=sqlc.arg(actor_id),updated_at=clock_timestamp() WHERE slot=true AND version=sqlc.arg(expected_version) RETURNING *;
-- name: AddDirectorEvent :exec
INSERT INTO organisation_director_events(id,designation_id,action,reason,before_snapshot,after_snapshot,created_at,created_by_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8);
-- name: DirectorHistory :many
SELECT * FROM organisation_director_events ORDER BY created_at DESC,id DESC LIMIT 200;

-- name: ReadWorkInventory :many
WITH assignments AS (
SELECT 'director'::text AS kind,p.id::text AS id,p.user_id::text AS owner_id,p.version::bigint AS version,'Organisation Director'::text AS title,'effective'::text AS state,'/dashboard?screen=director'::text AS link,jsonb_build_array('admin.governance.read')::jsonb AS required_permissions,''::text AS due_date FROM organisation_director_designation p WHERE p.is_active
) SELECT jsonb_build_object('Source','organisation_governance','Kind',kind,'ID',id,'OwnerID',owner_id,'Version',version,'Title',title,'State',state,'Link',link,'RequiredPermissions',required_permissions,'DueDate',due_date)::jsonb AS payload FROM assignments WHERE (sqlc.arg(owner_id)::text='' OR owner_id=sqlc.arg(owner_id)) AND (sqlc.arg(record_id)::text='' OR id=sqlc.arg(record_id)) AND (sqlc.arg(kind)::text='' OR kind=sqlc.arg(kind)) ORDER BY kind,id,owner_id LIMIT 10001;
