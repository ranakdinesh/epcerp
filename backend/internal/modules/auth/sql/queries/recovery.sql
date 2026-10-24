-- name: RecoveryUserByEmail :many
SELECT * FROM auth.users WHERE lower(email) = lower($1) LIMIT 2;

-- name: RecoveryUserID :one
SELECT user_id FROM auth.auth_password_recovery WHERE token_hash = $1;

-- name: RecoveryTokenValid :one
SELECT t.token_hash FROM auth.auth_password_recovery t JOIN auth.users u ON u.id=t.user_id
WHERE t.token_hash=$1 AND t.consumed_at IS NULL AND t.expires_at > clock_timestamp()
AND u.is_active AND u.updated_at <= t.created_at
FOR UPDATE OF t;

-- name: RecoveryRecentlyRequested :one
SELECT EXISTS (SELECT 1 FROM auth.auth_password_recovery WHERE user_id=$1 AND created_at > clock_timestamp() - interval '1 minute');

-- name: InsertRecoveryToken :exec
INSERT INTO auth.auth_password_recovery(token_hash,user_id,expires_at) VALUES ($1,$2,$3);

-- name: ConsumeRecoveryTokens :exec
UPDATE auth.auth_password_recovery SET consumed_at=clock_timestamp() WHERE user_id=$1 AND consumed_at IS NULL;
