-- name: GetProfile :one
SELECT * FROM auth.user_profile WHERE user_id=$1 AND is_active=true;
-- name: SaveProfile :one
INSERT INTO auth.user_profile(user_id,display_name,photo_object_key,photo_sha256,created_by_id,updated_by_id)
VALUES(sqlc.arg(user_id),coalesce(sqlc.narg(display_name)::text,''),sqlc.narg(photo_object_key)::text,sqlc.narg(photo_sha256)::text,sqlc.arg(user_id),sqlc.arg(user_id))
ON CONFLICT(user_id) DO UPDATE SET
 display_name=coalesce(sqlc.narg(display_name)::text,user_profile.display_name),
 photo=CASE WHEN sqlc.arg(replace_photo)::boolean THEN NULL ELSE user_profile.photo END,
 photo_object_key=CASE WHEN sqlc.arg(replace_photo)::boolean THEN sqlc.narg(photo_object_key)::text ELSE user_profile.photo_object_key END,
 photo_sha256=CASE WHEN sqlc.arg(replace_photo)::boolean THEN sqlc.narg(photo_sha256)::text ELSE user_profile.photo_sha256 END,
 version=user_profile.version+1, updated_at=now(),updated_by_id=excluded.updated_by_id
RETURNING *;

-- name: AdvancePersonalOrganisationVersion :exec
UPDATE auth.auth_reporting_lines SET version=version+1,updated_at=clock_timestamp(),updated_by_id=$1 WHERE user_id=$1;
-- name: RecordPersonalContactChange :exec
INSERT INTO auth.auth_personal_contact_changes(id,user_id,version,reason,before_snapshot,after_snapshot,actor_id)
VALUES($1,$2,$3,$4,$5,$6,$7);
