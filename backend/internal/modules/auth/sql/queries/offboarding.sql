-- name: OffboardingCases :many
SELECT c.* FROM auth.auth_offboarding_cases c JOIN auth.auth_reporting_lines r ON r.user_id=c.user_id
WHERE sqlc.arg(administrator)::boolean OR (c.manager_id=sqlc.arg(actor_id) AND r.manager_id=sqlc.arg(actor_id)) ORDER BY c.created_at DESC,c.id LIMIT 1000;
-- name: OffboardingCase :one
SELECT * FROM auth.auth_offboarding_cases WHERE id=$1;
-- name: SaveOffboardingCase :one
INSERT INTO auth.auth_offboarding_cases(id,user_id,manager_id,profile_version,state,reason,items,approval_by_id,approved_at,is_active,created_by_id,updated_by_id)
VALUES(sqlc.arg(id),sqlc.arg(user_id),sqlc.arg(manager_id),sqlc.arg(profile_version),sqlc.arg(state),sqlc.arg(reason),sqlc.arg(items),sqlc.narg(approval_by_id),sqlc.narg(approved_at),sqlc.arg(is_active),sqlc.arg(actor_id),sqlc.arg(actor_id))
ON CONFLICT(id) DO UPDATE SET manager_id=excluded.manager_id,profile_version=excluded.profile_version,state=excluded.state,items=excluded.items,approval_by_id=excluded.approval_by_id,approved_at=excluded.approved_at,is_active=excluded.is_active,version=auth_offboarding_cases.version+1,updated_at=clock_timestamp(),updated_by_id=excluded.updated_by_id
WHERE auth_offboarding_cases.version=sqlc.arg(expected_version) RETURNING *;
-- name: RecordOffboardingChange :exec
INSERT INTO auth.auth_offboarding_changes(id,case_id,version,action,reason,snapshot,created_by_id)
SELECT sqlc.arg(event_id),c.id,c.version,sqlc.arg(action),sqlc.arg(reason),to_jsonb(c),sqlc.arg(actor_id) FROM auth.auth_offboarding_cases c WHERE c.id=sqlc.arg(case_id);

-- name: OffboardingHistory :many
SELECT h.id,h.action,h.reason,h.created_by_id,h.version,h.created_at,COALESCE(nullif(trim(u.first_name||' '||u.last_name),''),u.username)::text AS actor_name FROM auth.auth_offboarding_changes h JOIN auth.users u ON u.id=h.created_by_id WHERE h.case_id=$1 ORDER BY h.version DESC LIMIT 1000;
