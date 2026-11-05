-- name: VendorDocumentsLockImport :exec
SELECT pg_advisory_xact_lock(3019,1);
-- name: VendorDocumentsListDocuments :many
SELECT d.version,d.id,d.supplier_id,d.doc_type,d.title,d.original_filename,d.content_type,d.byte_size,d.sha256,d.is_active,d.created_at,d.updated_at,d.created_by_id,d.updated_by_id,d.source_id,d.source_created_at,d.source_updated_at,coalesce(a.state,'')::text AS approval_state,coalesce(a.comment,'')::text AS approval_comment FROM supplier_documents d LEFT JOIN supplier_document_approvals a ON a.document_id=d.id AND a.document_version=d.version AND a.sha256=d.sha256 WHERE d.supplier_id=$1 ORDER BY d.created_at,d.id;
-- name: VendorDocumentsGetDocument :one
SELECT * FROM supplier_documents WHERE supplier_id=$1 AND id=$2;
-- name: VendorDocumentsSourceDocument :one
SELECT * FROM supplier_documents WHERE source_id=$1;
-- name: VendorDocumentsInsertDocument :one
INSERT INTO supplier_documents(id,supplier_id,doc_type,title,original_filename,content_type,byte_size,encrypted_content,sha256,is_active,created_by_id,updated_by_id,source_id,source_created_at,source_updated_at,object_bucket,object_key) VALUES(sqlc.arg(id),sqlc.arg(supplier_id),sqlc.arg(doc_type),sqlc.arg(title),sqlc.arg(filename),sqlc.arg(content_type),sqlc.arg(byte_size),sqlc.arg(encrypted_content),sqlc.arg(sha256),sqlc.arg(active),sqlc.arg(actor),sqlc.arg(actor),sqlc.arg(source_id),sqlc.arg(source_created_at),sqlc.arg(source_updated_at),sqlc.narg(object_bucket),sqlc.narg(object_key)) RETURNING *;

-- name: VendorDocumentsDocumentCreatedAt :one
SELECT created_at FROM supplier_documents WHERE id=$1;

-- name: VendorDocumentsArchiveDocument :one
UPDATE supplier_documents SET is_active=false,version=version+1,updated_at=now(),updated_by_id=sqlc.arg(actor) WHERE supplier_id=sqlc.arg(supplier) AND id=sqlc.arg(id) AND version=sqlc.arg(version) AND is_active=true RETURNING *;
-- name: VendorDocumentsActiveEvidence :many
SELECT * FROM supplier_documents WHERE supplier_id=$1 AND is_active ORDER BY id;

-- name: VendorDocumentsRecordChange :exec
INSERT INTO supplier_document_changes(id,document_id,version,action,reason,created_by_id) VALUES($1,$2,$3,$4,$5,$6);

-- name: VendorDocumentsNextInline :one
SELECT * FROM supplier_documents WHERE encrypted_content IS NOT NULL ORDER BY id LIMIT 1 FOR UPDATE;
-- name: VendorDocumentsRelocate :exec
UPDATE supplier_documents SET encrypted_content=NULL,object_bucket=$2,object_key=$3 WHERE id=$1 AND encrypted_content IS NOT NULL;
