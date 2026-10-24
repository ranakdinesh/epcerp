-- name: UsernameAvailable :one
SELECT NOT EXISTS(SELECT 1 FROM auth.users WHERE username=$1 AND id<>$2)::boolean;
-- name: SaveUsername :exec
UPDATE auth.users SET username=$2,updated_at=clock_timestamp(),updated_by_id=$1 WHERE id=$1;
-- name: RecordUsernameChange :exec
INSERT INTO auth.auth_username_changes(id,user_id,version,previous_username,username,created_by_id)
VALUES($1,$2,$3,$4,$5,$2);
-- name: BoundBootstrapUser :one
SELECT u.* FROM auth.users u JOIN auth.auth_bootstrap_identity b ON b.user_id=u.id WHERE b.singleton;
-- name: BindBootstrapUser :exec
INSERT INTO auth.auth_bootstrap_identity(singleton,user_id) VALUES(true,$1);
