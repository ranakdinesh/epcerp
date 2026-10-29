-- name: CreateTSCJob :one
INSERT INTO tender.tsc_jobs(id,draft_id,draft_version,actor_id,operation_key,mode,model,input_cipher,input_hash)
VALUES(sqlc.arg(id),sqlc.arg(draft_id),sqlc.arg(draft_version),sqlc.arg(actor_id),sqlc.arg(operation_key),sqlc.arg(mode),sqlc.arg(model),sqlc.arg(input_cipher),sqlc.arg(input_hash)) RETURNING *;
-- name: FindTSCOperation :one
SELECT * FROM tender.tsc_jobs WHERE draft_id=sqlc.arg(draft_id) AND actor_id=sqlc.arg(actor_id) AND operation_key=sqlc.arg(operation_key);
-- name: GetTSCJob :one
SELECT * FROM tender.tsc_jobs WHERE draft_id=sqlc.arg(draft_id) AND id=sqlc.arg(id) FOR UPDATE;
-- name: ListTSCJobs :many
SELECT id,draft_id,draft_version,actor_id,mode,model,state,version,attempts,error_code,created_at,updated_at FROM tender.tsc_jobs WHERE draft_id=$1 ORDER BY created_at DESC LIMIT 20;
-- name: CountTSCJobs :one
SELECT count(*) FROM tender.tsc_jobs WHERE draft_id=$1;
-- name: ClaimTSCJob :one
UPDATE tender.tsc_jobs SET state='processing',claim_id=sqlc.arg(claim_id),lease_until=clock_timestamp()+interval '2 minutes',attempts=attempts+1,version=version+1,updated_at=clock_timestamp()
WHERE id=(SELECT id FROM tender.tsc_jobs WHERE (state='queued' OR state='processing' AND lease_until<clock_timestamp()) AND attempts<3 ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING *;
-- name: ExpireTSCJobs :many
UPDATE tender.tsc_jobs SET state='failed',claim_id=NULL,lease_until=NULL,error_code='attempts_exhausted',version=version+1,updated_at=clock_timestamp() WHERE state='processing' AND lease_until<clock_timestamp() AND attempts>=3 RETURNING *;
-- name: RenewTSCJob :execrows
UPDATE tender.tsc_jobs SET lease_until=clock_timestamp()+interval '2 minutes' WHERE id=sqlc.arg(id) AND state='processing' AND claim_id=sqlc.arg(claim_id) AND lease_until>clock_timestamp();
-- name: FinishTSCJob :one
UPDATE tender.tsc_jobs SET state=sqlc.arg(state),claim_id=NULL,lease_until=NULL,error_code=sqlc.arg(error_code),result_cipher=sqlc.narg(result_cipher),result_hash=sqlc.narg(result_hash),version=version+1,updated_at=clock_timestamp()
WHERE id=sqlc.arg(id) AND claim_id=sqlc.arg(claim_id) AND state='processing' AND lease_until>clock_timestamp() RETURNING *;
-- name: EditTSCJob :one
UPDATE tender.tsc_jobs SET result_cipher=sqlc.arg(result_cipher),result_hash=sqlc.arg(result_hash),version=version+1,updated_at=clock_timestamp()
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) AND state='ready' RETURNING *;
-- name: InsertTSCEvent :exec
INSERT INTO tender.tsc_events(id,job_id,action,version,reason,result_cipher,result_hash,created_by_id)
VALUES(sqlc.arg(id),sqlc.arg(job_id),sqlc.arg(action),sqlc.arg(version),sqlc.arg(reason),sqlc.narg(result_cipher),sqlc.narg(result_hash),sqlc.arg(actor_id));
