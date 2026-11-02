-- name: SetAuthActor :exec
SELECT set_config('epcerp.auth_actor', $1::text, true);
-- name: AccessRevoked :one
SELECT EXISTS(SELECT 1 FROM auth.access_sessions WHERE token_hash=$1 AND revoked_at IS NOT NULL);
-- name: TrackAccess :exec
INSERT INTO auth.access_sessions(token_hash,user_id,family_id,expires_at) VALUES($1,$2,$3,$4);
-- name: RevokeAccessFamily :exec
UPDATE auth.access_sessions SET revoked_at=clock_timestamp() WHERE family_id=$1 AND revoked_at IS NULL;
-- name: ListSessions :many
SELECT family_id,min(created_at)::timestamptz AS created_at,max(expires_at)::timestamptz AS expires_at
FROM auth.refresh_tokens WHERE user_id=$1 AND revoked_at IS NULL AND expires_at>clock_timestamp()
GROUP BY family_id ORDER BY min(created_at) DESC;
-- name: SessionOwner :one
SELECT user_id FROM auth.refresh_tokens WHERE family_id=$1 LIMIT 1;
-- name: AccessFamily :one
SELECT family_id FROM auth.access_sessions WHERE token_hash=$1;
-- name: ListAdministrationChanges :many
SELECT * FROM auth.administration_changes WHERE (sqlc.narg(entity_id)::uuid IS NULL OR entity_id=sqlc.narg(entity_id)) ORDER BY id DESC LIMIT 100;
-- name: LoginBlocked :one
WITH stale AS (DELETE FROM auth.login_attempts WHERE identifier_hash IN (SELECT identifier_hash FROM auth.login_attempts WHERE window_started_at<clock_timestamp()-interval '1 day' LIMIT 100))
INSERT INTO auth.login_attempts(identifier_hash) VALUES($1) ON CONFLICT(identifier_hash) DO UPDATE SET identifier_hash=EXCLUDED.identifier_hash
RETURNING COALESCE(blocked_until>clock_timestamp(),false)::boolean AS blocked;
-- name: FailLogin :exec
UPDATE auth.login_attempts SET
 failures=CASE WHEN window_started_at<clock_timestamp()-interval '15 minutes' THEN 1 ELSE failures+1 END,
 blocked_until=CASE WHEN window_started_at>=clock_timestamp()-interval '15 minutes' AND failures>=4 THEN clock_timestamp()+interval '15 minutes' ELSE NULL END,
 window_started_at=CASE WHEN window_started_at<clock_timestamp()-interval '15 minutes' THEN clock_timestamp() ELSE window_started_at END
WHERE identifier_hash=$1;
-- name: ClearLoginFailures :exec
DELETE FROM auth.login_attempts WHERE identifier_hash=$1;
-- name: CancelEmailChanges :exec
UPDATE auth.email_changes SET consumed_at=clock_timestamp() WHERE user_id=$1 AND consumed_at IS NULL;
-- name: InsertEmailChange :exec
INSERT INTO auth.email_changes(token_hash,user_id,email,previous_email,credential_version,created_by_id,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7);
-- name: LockEmailChange :one
SELECT * FROM auth.email_changes WHERE token_hash=$1 FOR UPDATE;

-- name: RevokeAllAccess :exec
WITH refresh AS (UPDATE auth.refresh_tokens SET revoked_at=clock_timestamp() WHERE auth.refresh_tokens.user_id=$1 AND revoked_at IS NULL)
UPDATE auth.access_sessions SET revoked_at=clock_timestamp() WHERE auth.access_sessions.user_id=$1 AND revoked_at IS NULL;

-- name: CancelCredentialForUser :exec
UPDATE auth.auth_credential_emails SET state='cancelled',payload=NULL,updated_at=clock_timestamp() WHERE user_id=$1 AND state IN ('pending','sending');
