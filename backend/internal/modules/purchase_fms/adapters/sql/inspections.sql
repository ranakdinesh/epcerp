-- name: InspectionsGetInspection :one
SELECT * FROM purchase_inspections WHERE project_id=sqlc.arg(project_id) AND package_id=sqlc.arg(package_id) FOR UPDATE;
-- name: InspectionsInsertInspection :one
INSERT INTO purchase_inspections(id,project_id,package_id,order_id,order_version,acceptance_id,acceptance_version,technical_id,technical_version,revision,version,status,source_snapshot,data,created_at,updated_at,created_by_id,updated_by_id)
VALUES(sqlc.arg(id),sqlc.arg(project_id),sqlc.arg(package_id),sqlc.arg(order_id),sqlc.arg(order_version),sqlc.arg(acceptance_id),sqlc.arg(acceptance_version),sqlc.arg(technical_id),sqlc.arg(technical_version),sqlc.arg(revision),1,sqlc.arg(status),sqlc.arg(source_snapshot),sqlc.arg(data),sqlc.arg(at),sqlc.arg(at),sqlc.arg(actor_id),sqlc.arg(actor_id)) RETURNING *;
-- name: InspectionsUpdateInspection :one
UPDATE purchase_inspections SET version=version+1,status=sqlc.arg(status),source_snapshot=sqlc.arg(source_snapshot),order_id=sqlc.arg(order_id),order_version=sqlc.arg(order_version),acceptance_id=sqlc.arg(acceptance_id),acceptance_version=sqlc.arg(acceptance_version),technical_id=sqlc.arg(technical_id),technical_version=sqlc.arg(technical_version),revision=sqlc.arg(revision),data=sqlc.arg(data),updated_at=sqlc.arg(at),updated_by_id=sqlc.arg(actor_id)
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) AND is_active RETURNING *;
-- name: InspectionsEvent :exec
INSERT INTO purchase_inspection_events(id,subject_id,action,reason,before_snapshot,after_snapshot,created_at,created_by_id)
VALUES(sqlc.arg(id),sqlc.arg(inspection_id),sqlc.arg(action),sqlc.arg(reason),sqlc.arg(before_snapshot),sqlc.arg(after_snapshot),sqlc.arg(at),sqlc.arg(actor_id));
-- name: InspectionsHistory :many
SELECT * FROM purchase_inspection_events WHERE subject_id=$1 ORDER BY created_at DESC,id DESC;
