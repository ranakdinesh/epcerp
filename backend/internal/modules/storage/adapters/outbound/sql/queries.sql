-- name: InsertDocument :one
INSERT INTO stored_documents(id,entity_type,entity_id,doc_type,title,original_filename,content_type,byte_size,storage_bucket,storage_key,sha256_hash,is_encrypted,encryption_algorithm,is_active,created_at,created_by_id,updated_at,updated_by_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,true,$12,true,$13,$14,$13,$14) RETURNING *;
-- name: GetDocument :one
SELECT * FROM stored_documents WHERE id=$1;
-- name: LockDocument :one
SELECT * FROM stored_documents WHERE id=$1 FOR UPDATE;
-- name: ListDocuments :many
SELECT * FROM stored_documents WHERE entity_type=$1 AND entity_id=$2 AND is_active ORDER BY created_at DESC;
-- name: DeactivateDocument :one
UPDATE stored_documents SET is_active=false,updated_at=now(),updated_by_id=$2 WHERE id=$1 RETURNING *;
-- name: InsertEvent :exec
INSERT INTO stored_document_events(id,document_id,action,actor_id,metadata_snapshot) VALUES($1,$2,$3,$4,$5);
