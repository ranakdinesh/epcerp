-- name: GRNsGetGRN :one
SELECT * FROM purchase_grns WHERE project_id=sqlc.arg(project_id) AND package_id=sqlc.arg(package_id) FOR UPDATE;
-- name: GRNsInsertGRN :one
INSERT INTO purchase_grns(id,project_id,package_id,receipt_id,receipt_version,revision,version,status,source_snapshot,data,created_at,updated_at,created_by_id,updated_by_id)
VALUES(sqlc.arg(id),sqlc.arg(project_id),sqlc.arg(package_id),sqlc.arg(receipt_id),sqlc.arg(receipt_version),sqlc.arg(revision),1,sqlc.arg(status),sqlc.arg(source_snapshot),sqlc.arg(data),sqlc.arg(at),sqlc.arg(at),sqlc.arg(actor_id),sqlc.arg(actor_id)) RETURNING *;
-- name: GRNsUpdateGRN :one
UPDATE purchase_grns SET version=version+1,status=sqlc.arg(status),source_snapshot=sqlc.arg(source_snapshot),receipt_id=sqlc.arg(receipt_id),receipt_version=sqlc.arg(receipt_version),revision=sqlc.arg(revision),data=sqlc.arg(data),updated_at=sqlc.arg(at),updated_by_id=sqlc.arg(actor_id)
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) AND is_active RETURNING *;
-- name: GRNsEvent :exec
INSERT INTO purchase_grns_events(id,subject_id,action,reason,before_snapshot,after_snapshot,created_at,created_by_id)
VALUES(sqlc.arg(id),sqlc.arg(grn_id),sqlc.arg(action),sqlc.arg(reason),sqlc.arg(before_snapshot),sqlc.arg(after_snapshot),sqlc.arg(at),sqlc.arg(actor_id));
-- name: GRNsHistory :many
SELECT * FROM purchase_grns_events WHERE subject_id=$1 ORDER BY created_at DESC,id DESC;
