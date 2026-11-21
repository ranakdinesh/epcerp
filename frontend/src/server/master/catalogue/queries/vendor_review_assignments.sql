-- name: VendorReviewLockSupplier :one
SELECT id FROM suppliers WHERE id=$1 FOR UPDATE;
-- name: VendorReviewAssignmentGet :one
SELECT a.*,u.username FROM supplier_review_assignments a JOIN auth.users u ON u.id=a.user_id WHERE supplier_id=$1;
-- name: VendorReviewAssignmentSave :exec
INSERT INTO supplier_review_assignments(id,supplier_id,user_id,version,is_active,created_by_id,updated_by_id) VALUES($1,$2,$3,$4,$5,$6,$6)
ON CONFLICT(supplier_id) DO UPDATE SET user_id=EXCLUDED.user_id,version=EXCLUDED.version,is_active=EXCLUDED.is_active,updated_by_id=EXCLUDED.updated_by_id,updated_at=now();
-- name: VendorReviewAssignmentJournal :exec
INSERT INTO supplier_review_assignment_changes(id,supplier_id,version,reason,snapshot,created_by_id) VALUES($1,$2,$3,$4,$5,$6);
-- name: VendorReviewOwnAssignments :many
SELECT s.id,s.code,s.name FROM supplier_review_assignments a JOIN suppliers s ON s.id=a.supplier_id WHERE a.user_id=$1 AND a.is_active AND s.is_active ORDER BY s.code;
