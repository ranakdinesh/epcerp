-- name: DocumentApprovalGet :one
SELECT * FROM supplier_document_approvals WHERE document_id=$1 FOR UPDATE;
-- name: DocumentApprovalSave :one
INSERT INTO supplier_document_approvals(document_id,supplier_id,document_version,sha256,state,comment,version,created_by_id,updated_by_id)
VALUES($1,$2,$3,$4,$5,$6,1,$7,$7)
ON CONFLICT(document_id) DO UPDATE SET document_version=EXCLUDED.document_version,sha256=EXCLUDED.sha256,state=EXCLUDED.state,comment=EXCLUDED.comment,version=supplier_document_approvals.version+1,updated_at=now(),updated_by_id=EXCLUDED.updated_by_id
RETURNING *;
-- name: DocumentApprovalJournal :exec
INSERT INTO supplier_document_approval_changes(id,document_id,version,action,snapshot,created_by_id)VALUES($1,$2,$3,$4,$5,$6);
-- name: DocumentTaskCreate :exec
INSERT INTO supplier_document_tasks(id,document_id,supplier_id,user_id,kind,comment,created_by_id,updated_by_id)VALUES($1,$2,$3,$4,$5,$6,$7,$7)
ON CONFLICT(document_id,kind)WHERE completed_at IS NULL DO NOTHING;
-- name: DocumentTasksComplete :exec
UPDATE supplier_document_tasks SET completed_at=now(),outcome=$3,comment=$4,updated_at=now(),updated_by_id=$5
WHERE document_id=$1 AND kind=$2 AND completed_at IS NULL;
-- name: DocumentTasksOwn :many
SELECT t.*,s.code,s.name,d.title,d.original_filename,d.doc_type FROM supplier_document_tasks t JOIN suppliers s ON s.id=t.supplier_id JOIN supplier_documents d ON d.id=t.document_id
WHERE t.user_id=$1 AND t.is_active ORDER BY t.created_at DESC,t.id;
-- name: DocumentApprovalState :one
SELECT state,comment FROM supplier_document_approvals WHERE document_id=$1 AND document_version=$2 AND sha256=$3 AND is_active;
-- name: DocumentTasksReassign :exec
UPDATE supplier_document_tasks SET user_id=$2,updated_by_id=$3,updated_at=now() WHERE supplier_id=$1 AND kind='review' AND completed_at IS NULL;
