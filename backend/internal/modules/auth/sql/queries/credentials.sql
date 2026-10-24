-- name: SetTemporaryPassword :exec
UPDATE auth.users SET password_hash=$2,must_change_password=true,temporary_password_expires_at=clock_timestamp()+interval '24 hours',credential_version=credential_version+1,updated_at=clock_timestamp(),updated_by_id=$3 WHERE id=$1;
-- name: QueueCredentialEmail :exec
INSERT INTO auth.auth_credential_emails(id,user_id,credential_version,payload,payload_hash,expires_at,created_by_id,updated_by_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$7);
-- name: CancelStaleCredentialEmails :exec
UPDATE auth.auth_credential_emails e SET state=CASE WHEN e.attempts>=5 THEN 'failed' ELSE 'cancelled' END,payload=NULL,updated_at=clock_timestamp(),updated_by_id=NULL
WHERE e.state IN ('pending','sending') AND (e.expires_at<=clock_timestamp() OR (e.attempts>=5 AND e.state='sending' AND e.lease_until<clock_timestamp()) OR NOT EXISTS (SELECT 1 FROM auth.users u WHERE u.id=e.user_id AND u.is_active AND u.must_change_password AND u.credential_version=e.credential_version));
-- name: ClaimCredentialEmail :one
UPDATE auth.auth_credential_emails SET state='sending',attempts=attempts+1,lease_id=$1,lease_until=clock_timestamp()+interval '60 seconds',updated_at=clock_timestamp(),updated_by_id=NULL
WHERE id=(SELECT e.id FROM auth.auth_credential_emails e JOIN auth.users u ON u.id=e.user_id
 WHERE u.is_active AND u.must_change_password AND u.credential_version=e.credential_version
 AND e.expires_at>clock_timestamp() AND e.attempts<5 AND
 ((e.state='pending' AND e.next_attempt_at<=clock_timestamp()) OR (e.state='sending' AND e.lease_until<clock_timestamp()))
 ORDER BY e.created_at LIMIT 1 FOR UPDATE OF e SKIP LOCKED) RETURNING *;
-- name: FinishCredentialEmail :exec
UPDATE auth.auth_credential_emails SET
 state=CASE WHEN sqlc.arg(delivered)::boolean THEN 'delivered' WHEN attempts>=5 OR expires_at<=clock_timestamp() THEN 'failed' ELSE 'pending' END,
 payload=CASE WHEN sqlc.arg(delivered)::boolean OR attempts>=5 OR expires_at<=clock_timestamp() THEN NULL ELSE payload END,
 lease_until=NULL,next_attempt_at=clock_timestamp()+interval '1 minute'*attempts,updated_at=clock_timestamp(),updated_by_id=NULL
WHERE id=sqlc.arg(id) AND lease_id=sqlc.arg(lease_id) AND state='sending';
-- name: CredentialEmailStatus :one
SELECT state FROM auth.auth_credential_emails WHERE user_id=$1 ORDER BY created_at DESC LIMIT 1;

-- name: RecordCredentialChange :exec
INSERT INTO auth.auth_credential_changes(id,user_id,actor_id,action,reason,credential_version)
SELECT sqlc.arg(event_id),u.id,sqlc.arg(actor_id),sqlc.arg(action),sqlc.arg(reason),u.credential_version FROM auth.users u WHERE u.id=sqlc.arg(user_id);

-- name: CancelCredentialEmail :exec
UPDATE auth.auth_credential_emails SET state='cancelled',payload=NULL,updated_at=clock_timestamp(),updated_by_id=NULL
WHERE id=$1 AND lease_id=$2 AND state='sending';
