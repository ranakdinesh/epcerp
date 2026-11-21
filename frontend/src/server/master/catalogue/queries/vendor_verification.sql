-- name: VendorVerificationGetReview :one
SELECT * FROM supplier_verifications WHERE supplier_id=$1;
-- name: VendorVerificationSaveReview :one
INSERT INTO supplier_verifications(supplier_id,version,fingerprint,decisions,reason,created_by_id,updated_by_id) VALUES(sqlc.arg(supplier_id),sqlc.arg(version),sqlc.arg(fingerprint),sqlc.arg(decisions),sqlc.arg(reason),sqlc.arg(actor),sqlc.arg(actor)) ON CONFLICT(supplier_id) DO UPDATE SET version=EXCLUDED.version,fingerprint=EXCLUDED.fingerprint,decisions=EXCLUDED.decisions,reason=EXCLUDED.reason,updated_by_id=EXCLUDED.updated_by_id,updated_at=now() RETURNING *;

-- name: VendorVerificationRecordChange :exec
INSERT INTO supplier_verification_history(id,supplier_id,version,snapshot,created_by_id) VALUES($1,$2,$3,$4,$5);
