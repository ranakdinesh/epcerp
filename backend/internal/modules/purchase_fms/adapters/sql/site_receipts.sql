-- name: SiteReceiptsGetReceipt :one
SELECT * FROM purchase_site_receipts WHERE project_id=sqlc.arg(project_id) AND package_id=sqlc.arg(package_id) FOR UPDATE;
-- name: SiteReceiptsInsertReceipt :one
INSERT INTO purchase_site_receipts(id,project_id,package_id,dispatch_id,dispatch_version,revision,version,status,source_snapshot,data,created_at,updated_at,created_by_id,updated_by_id)
VALUES(sqlc.arg(id),sqlc.arg(project_id),sqlc.arg(package_id),sqlc.arg(dispatch_id),sqlc.arg(dispatch_version),sqlc.arg(revision),1,sqlc.arg(status),sqlc.arg(source_snapshot),sqlc.arg(data),sqlc.arg(at),sqlc.arg(at),sqlc.arg(actor_id),sqlc.arg(actor_id)) RETURNING *;
-- name: SiteReceiptsUpdateReceipt :one
UPDATE purchase_site_receipts SET version=version+1,status=sqlc.arg(status),source_snapshot=sqlc.arg(source_snapshot),dispatch_id=sqlc.arg(dispatch_id),dispatch_version=sqlc.arg(dispatch_version),revision=sqlc.arg(revision),data=sqlc.arg(data),updated_at=sqlc.arg(at),updated_by_id=sqlc.arg(actor_id)
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) AND is_active RETURNING *;
-- name: SiteReceiptsEvent :exec
INSERT INTO purchase_site_receipts_events(id,subject_id,action,reason,before_snapshot,after_snapshot,created_at,created_by_id)
VALUES(sqlc.arg(id),sqlc.arg(receipt_id),sqlc.arg(action),sqlc.arg(reason),sqlc.arg(before_snapshot),sqlc.arg(after_snapshot),sqlc.arg(at),sqlc.arg(actor_id));
-- name: SiteReceiptsHistory :many
SELECT * FROM purchase_site_receipts_events WHERE subject_id=$1 ORDER BY created_at DESC,id DESC;
