-- name: ManufacturersLockNames :exec
SELECT pg_advisory_xact_lock(3017,1);
-- name: ManufacturersListManufacturers :many
SELECT * FROM manufacturers ORDER BY name;
-- name: ManufacturersGetManufacturer :one
SELECT * FROM manufacturers WHERE id=$1;
-- name: ManufacturersLockManufacturer :one
SELECT * FROM manufacturers WHERE id=$1 FOR UPDATE;
-- name: ManufacturersInsertManufacturer :one
INSERT INTO manufacturers(id,code,name,data,is_active,version,created_by_id,updated_by_id) VALUES(sqlc.arg(id),sqlc.arg(code),sqlc.arg(name),sqlc.arg(data),sqlc.arg(is_active),1,sqlc.arg(actor_id),sqlc.arg(actor_id)) RETURNING *;
-- name: ManufacturersProfileManufacturer :one
UPDATE manufacturers SET name=sqlc.arg(name),data=sqlc.arg(data),version=sqlc.arg(version),updated_by_id=sqlc.arg(actor_id),updated_at=clock_timestamp() WHERE id=sqlc.arg(id) RETURNING *;
-- name: ManufacturersStatusManufacturer :one
UPDATE manufacturers SET is_active=sqlc.arg(active),version=sqlc.arg(version),updated_by_id=sqlc.arg(actor_id),updated_at=clock_timestamp() WHERE id=sqlc.arg(id) RETURNING *;
-- name: ManufacturersInsertEvent :exec
INSERT INTO manufacturers_events(id,subject_id,action,reason,"before","after",created_by_id) VALUES(sqlc.arg(id),sqlc.arg(subject_id),sqlc.arg(action),sqlc.arg(reason),sqlc.arg(before_snapshot),sqlc.arg(after_snapshot),sqlc.arg(actor_id));
-- name: ManufacturersHistory :many
SELECT * FROM manufacturers_events WHERE subject_id=$1 ORDER BY created_at DESC;

-- name: ManufacturersFindImport :one
SELECT go_id,source_fingerprint FROM manufacturer_imports WHERE source_id= @source_id;
-- name: ManufacturersInsertImport :exec
INSERT INTO manufacturer_imports(source_id,go_id,source_snapshot,source_fingerprint,created_by_id,updated_by_id)VALUES(@source_id,@go_id,@source_snapshot,@source_fingerprint,@actor_id,@actor_id);
