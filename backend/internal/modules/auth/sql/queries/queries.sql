-- name: AdvisoryLock :exec
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(lock_name)::text, 0));

-- name: GetUser :one
SELECT * FROM auth.users WHERE id=$1;
-- name: LockUser :one
SELECT * FROM auth.users WHERE id=$1 FOR UPDATE;
-- name: FindUser :many
SELECT * FROM auth.users WHERE username=sqlc.arg(identifier)::text OR email=lower(sqlc.arg(identifier)::text) LIMIT 2;
-- name: ListUserRoles :many
SELECT r.*, COALESCE((SELECT jsonb_agg(p.code ORDER BY p.code) FROM auth.permissions p JOIN auth.role_permissions rp ON rp.permission_id=p.id WHERE rp.role_id=r.id),'[]'::jsonb)::jsonb AS permission_codes
FROM auth.roles r JOIN auth.user_roles ur ON ur.role_id=r.id WHERE ur.user_id=$1 ORDER BY r.name;
-- name: ListRoles :many
SELECT * FROM auth.roles ORDER BY name;
-- name: GetRole :one
SELECT * FROM auth.roles WHERE id=$1;
-- name: LockAssignmentBundle :one
SELECT * FROM auth.roles WHERE id=$1 FOR SHARE;
-- name: FindRole :one
SELECT * FROM auth.roles WHERE name=$1;
-- name: RolePermissions :many
SELECT p.code FROM auth.permissions p JOIN auth.role_permissions rp ON rp.permission_id=p.id WHERE rp.role_id=$1 ORDER BY p.code;
-- name: RegisteredPermissions :many
SELECT code FROM auth.permissions ORDER BY code;
-- name: FilterUsers :many
SELECT * FROM auth.users u WHERE
(sqlc.narg(active)::boolean IS NULL OR u.is_active=sqlc.narg(active)) AND
(sqlc.arg(search)::text='' OR lower(u.username) LIKE '%' || lower(sqlc.arg(search)) || '%' OR lower(u.email) LIKE '%' || lower(sqlc.arg(search)) || '%') AND
(sqlc.arg(role_name)::text='' OR EXISTS(SELECT 1 FROM auth.user_roles ur JOIN auth.roles r ON r.id=ur.role_id WHERE ur.user_id=u.id AND r.name=sqlc.arg(role_name)))
ORDER BY CASE WHEN sqlc.arg(descending)::boolean THEN username END DESC, CASE WHEN NOT sqlc.arg(descending)::boolean THEN username END ASC
LIMIT sqlc.arg(page_size)::integer OFFSET sqlc.arg(page_offset)::bigint;
-- name: CountUsers :one
SELECT count(*) FROM auth.users u WHERE
(sqlc.narg(active)::boolean IS NULL OR u.is_active=sqlc.narg(active)) AND
(sqlc.arg(search)::text='' OR lower(u.username) LIKE '%' || lower(sqlc.arg(search)) || '%' OR lower(u.email) LIKE '%' || lower(sqlc.arg(search)) || '%') AND
(sqlc.arg(role_name)::text='' OR EXISTS(SELECT 1 FROM auth.user_roles ur JOIN auth.roles r ON r.id=ur.role_id WHERE ur.user_id=u.id AND r.name=sqlc.arg(role_name)));
-- name: InsertUser :one
INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser,created_by_id,updated_by_id,first_name,last_name,mobile)
VALUES($1,$2,$3,$4,true,sqlc.arg(is_superuser),$5,$5,$6,$7,$8) RETURNING *;
-- name: UpdateUser :exec
UPDATE auth.users SET email=COALESCE(sqlc.narg(email)::text,email), updated_by_id=sqlc.arg(actor_id),updated_at=now() WHERE id=sqlc.arg(id);
-- name: ClearUserRoles :exec
DELETE FROM auth.user_roles WHERE user_id=$1;
-- name: AssignRole :exec
INSERT INTO auth.user_roles(user_id,role_id) VALUES($1,$2) ON CONFLICT DO NOTHING;
-- name: SetPassword :exec
UPDATE auth.users SET password_hash=$2,must_change_password=false,temporary_password_expires_at=NULL,credential_version=credential_version+1,updated_by_id=$3,updated_at=clock_timestamp() WHERE id=$1;
-- name: DeactivateUser :exec
UPDATE auth.users SET is_active=false,updated_by_id=$2,updated_at=now() WHERE id=$1;
-- name: CountActiveSuperusers :one
SELECT count(*) FROM auth.users WHERE is_active AND is_superuser;
-- name: CountSuperusers :one
SELECT count(*) FROM auth.users WHERE is_superuser;
-- name: InsertRole :one
INSERT INTO auth.roles(id,name,description,is_active,created_by_id,updated_by_id) VALUES($1,$2,$3,true,$4,$4) RETURNING *;
-- name: UpdateRole :exec
UPDATE auth.roles SET description=COALESCE(sqlc.narg(description)::text,description),is_active=COALESCE(sqlc.narg(active)::boolean,is_active),updated_by_id=sqlc.arg(actor_id),updated_at=now() WHERE id=sqlc.arg(id);
-- name: ClearRolePermissions :exec
DELETE FROM auth.role_permissions WHERE role_id=$1;
-- name: GrantPermission :exec
INSERT INTO auth.role_permissions(role_id,permission_id) SELECT $1,id FROM auth.permissions WHERE code=$2 ON CONFLICT DO NOTHING;
-- name: InsertRefresh :exec
INSERT INTO auth.refresh_tokens(id,user_id,token_hash,family_id,expires_at) VALUES($1,$2,$3,$4,$5);
-- name: GetRefresh :one
SELECT * FROM auth.refresh_tokens WHERE token_hash=$1;
-- name: LockRefresh :one
SELECT * FROM auth.refresh_tokens WHERE token_hash=$1 FOR UPDATE;
-- name: RevokeRefresh :exec
UPDATE auth.refresh_tokens SET revoked_at=now() WHERE token_hash=$1 AND revoked_at IS NULL;
-- name: RevokeFamily :exec
UPDATE auth.refresh_tokens SET revoked_at=now() WHERE family_id=$1 AND revoked_at IS NULL;
-- name: RevokeUserSessions :exec
UPDATE auth.refresh_tokens SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL;
-- name: RecordActivity :exec
INSERT INTO auth.user_activity(user_id,current_page,last_seen_at) VALUES($1,$2,now()) ON CONFLICT(user_id) DO UPDATE SET current_page=excluded.current_page,last_seen_at=excluded.last_seen_at;
-- name: OnlineActivity :many
SELECT a.user_id,u.username,a.current_page,a.last_seen_at FROM auth.user_activity a JOIN auth.users u ON u.id=a.user_id WHERE u.is_active AND a.last_seen_at >= sqlc.arg(since)::timestamptz ORDER BY a.last_seen_at DESC;
-- name: ReconcilePermission :exec
INSERT INTO auth.permissions(id,code,description) VALUES($1,$2,$3) ON CONFLICT(code) DO UPDATE SET description=excluded.description;
-- name: EnsureBootstrapRole :one
INSERT INTO auth.roles(id,name,description,is_active) VALUES($1,'superadmin','Platform super administrator',true) ON CONFLICT(name) DO UPDATE SET name=roles.name RETURNING id;
-- name: FindBootstrapUser :one
SELECT * FROM auth.users WHERE username=$1;
-- name: RepairBootstrapUser :exec
UPDATE auth.users SET is_superuser=true,updated_at=now() WHERE id=$1;
-- name: InsertBootstrapUser :one
INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,$2,$3,$4,true,true) RETURNING id;

-- name: AssignmentCandidate :one
SELECT u.id,(u.is_active AND NOT u.must_change_password)::boolean AS ready,CASE WHEN trim(u.first_name||' '||u.last_name)<>'' THEN trim(u.first_name||' '||u.last_name)||' ('||u.username||')' ELSE u.username END::text AS username,u.is_active,u.is_superuser,
 COALESCE((SELECT jsonb_agg(DISTINCT p.code) FROM auth.user_roles ur JOIN auth.roles r ON r.id=ur.role_id AND r.is_active JOIN auth.role_permissions rp ON rp.role_id=r.id JOIN auth.permissions p ON p.id=rp.permission_id WHERE ur.user_id=u.id),'[]'::jsonb)::jsonb AS permission_codes
FROM auth.users u WHERE u.id=$1 FOR SHARE OF u;
-- name: AssignmentCandidates :many
SELECT u.id,(u.is_active AND NOT u.must_change_password)::boolean AS ready,CASE WHEN trim(u.first_name||' '||u.last_name)<>'' THEN trim(u.first_name||' '||u.last_name)||' ('||u.username||')' ELSE u.username END::text AS username,u.is_active,u.is_superuser,
 COALESCE((SELECT jsonb_agg(DISTINCT p.code) FROM auth.user_roles ur JOIN auth.roles r ON r.id=ur.role_id AND r.is_active JOIN auth.role_permissions rp ON rp.role_id=r.id JOIN auth.permissions p ON p.id=rp.permission_id WHERE ur.user_id=u.id),'[]'::jsonb)::jsonb AS permission_codes
FROM auth.users u WHERE u.is_active ORDER BY u.username;

-- name: AdvanceAccountVersion :one
UPDATE auth.users SET account_version=account_version+1 WHERE id=$1 RETURNING account_version,updated_at;
