-- name: VendorProfileLockImport :exec
SELECT pg_advisory_xact_lock(3018,1);
-- name: VendorProfileListSuppliers :many
SELECT * FROM suppliers ORDER BY name,id;
-- name: VendorProfileSourceSupplier :one
SELECT * FROM suppliers WHERE source_id=$1;
-- name: VendorProfileGetSupplier :one
SELECT * FROM suppliers WHERE id=$1 FOR UPDATE;
-- name: VendorProfileSupplierExists :one
SELECT id FROM suppliers WHERE id=$1 FOR UPDATE;
-- name: VendorProfileInsertSupplier :one
INSERT INTO suppliers(id,code,name,data,is_active,created_by_id,updated_by_id,source_id,source_version,source_created_at,source_updated_at) VALUES(sqlc.arg(id),sqlc.arg(code),sqlc.arg(name),sqlc.arg(data),sqlc.arg(active),sqlc.arg(actor),sqlc.arg(actor),sqlc.arg(source_id),sqlc.arg(source_version),sqlc.arg(source_created_at),sqlc.arg(source_updated_at)) RETURNING *;

-- name: VendorProfileUpdateSupplier :one
UPDATE suppliers SET name=sqlc.arg(name),data=sqlc.arg(data),is_active=sqlc.arg(active),version=version+1,updated_by_id=sqlc.arg(actor),updated_at=now() WHERE id=sqlc.arg(id) AND version=sqlc.arg(previous_version) RETURNING *;

-- name: VendorProfileRecordChange :exec
INSERT INTO supplier_changes(id,supplier_id,version,action,reason,snapshot,created_by_id) VALUES($1,$2,$3,$4,$5,$6,$7);
