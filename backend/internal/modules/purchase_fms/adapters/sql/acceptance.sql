-- name: AcceptanceGetAcceptance :one
SELECT * FROM purchase_acceptances WHERE project_id=sqlc.arg(project_id) AND package_id=sqlc.arg(package_id) FOR UPDATE;
-- name: AcceptanceInsertAcceptance :one
INSERT INTO purchase_acceptances(id,project_id,package_id,order_id,order_version,version,status,order_snapshot,data,created_at,updated_at,created_by_id,updated_by_id)
VALUES(sqlc.arg(id),sqlc.arg(project_id),sqlc.arg(package_id),sqlc.arg(order_id),sqlc.arg(order_version),1,sqlc.arg(status),sqlc.arg(order_snapshot),sqlc.arg(data),sqlc.arg(at),sqlc.arg(at),sqlc.arg(actor_id),sqlc.arg(actor_id)) RETURNING *;
-- name: AcceptanceUpdateAcceptance :one
UPDATE purchase_acceptances SET version=version+1,status=sqlc.arg(status),data=sqlc.arg(data),updated_at=sqlc.arg(at),updated_by_id=sqlc.arg(actor_id)
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) AND is_active RETURNING *;
-- name: AcceptanceEvent :exec
INSERT INTO purchase_acceptance_events(id,subject_id,action,reason,before_snapshot,after_snapshot,created_at,created_by_id)
VALUES(sqlc.arg(id),sqlc.arg(acceptance_id),sqlc.arg(action),sqlc.arg(reason),sqlc.arg(before_snapshot),sqlc.arg(after_snapshot),sqlc.arg(at),sqlc.arg(actor_id));
-- name: AcceptanceHistory :many
SELECT * FROM purchase_acceptance_events WHERE subject_id=$1 ORDER BY created_at DESC,id DESC;
