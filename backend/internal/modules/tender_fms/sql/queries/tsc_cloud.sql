-- name: LockTSCBudget :exec
SELECT id FROM tender.document_drafts WHERE id=$1 FOR UPDATE;
-- name: GetTSCBudget :one
SELECT COALESCE(sum(reserved_paise),0)::bigint FROM tender.tsc_cloud_calls WHERE draft_id=$1;
-- name: FindTSCCloudCall :one
SELECT * FROM tender.tsc_cloud_calls WHERE draft_id=$1 AND request_key=$2;
-- name: ReserveTSCCloudCall :one
INSERT INTO tender.tsc_cloud_calls(id,draft_id,job_id,actor_id,request_key,fingerprint,provider,model,reserved_paise)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING *;
-- name: CompleteTSCCloudCall :execrows
UPDATE tender.tsc_cloud_calls SET state=$2,error_code=$3,result_cipher=$4,result_hash=$5,updated_at=clock_timestamp()
WHERE id=$1 AND state='pending';
