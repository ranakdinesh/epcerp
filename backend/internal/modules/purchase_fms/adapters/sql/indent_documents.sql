-- name: IndentDocumentsListDocuments :many
SELECT * FROM purchase_indent_documents WHERE project_id= @project_id AND bo_id= @bo_id ORDER BY created_at,id;
-- name: IndentDocumentsInsertDocument :one
INSERT INTO purchase_indent_documents(id,project_id,bo_id,group_id,vendor_id,scope_revision,version,data,is_active,created_by_id,updated_by_id)VALUES(@id,@project_id,@bo_id,@group_id,@vendor_id,@scope_revision,1,@data,true,@actor_id,@actor_id)RETURNING *;
-- name: IndentDocumentsUpdateDocument :one
UPDATE purchase_indent_documents SET version=version+1,data= @data,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE id= @id AND project_id= @project_id AND bo_id= @bo_id AND version= @expected_version RETURNING *;
-- name: IndentDocumentsInsertDocumentEvent :exec
INSERT INTO purchase_indent_document_events(id,document_id,action,reason,before_snapshot,after_snapshot,created_by_id)VALUES(@id,@document_id,@action,@reason,@before_snapshot,@after_snapshot,@actor_id);
-- name: IndentDocumentsDocumentHistory :many
SELECT e.* FROM purchase_indent_document_events e JOIN purchase_indent_documents d ON d.id=e.document_id WHERE d.project_id= @project_id AND d.bo_id= @bo_id ORDER BY e.created_at DESC,e.id LIMIT 200;
-- name: IndentDocumentsCreateDispatchNotice :exec
INSERT INTO purchase_indent_dispatch_notices(id,document_id,recipient_id,created_by_id)VALUES(@id,@document_id,@recipient_id,@actor_id)ON CONFLICT(document_id,recipient_id)DO NOTHING;
-- name: IndentDocumentsResolveDispatchNotices :exec
UPDATE purchase_indent_dispatch_notices SET resolved_at=clock_timestamp(),updated_by_id= @actor_id WHERE document_id= @document_id AND resolved_at IS NULL;
-- name: IndentDocumentsDispatchNotices :many
SELECT n.id,d.project_id,d.bo_id,d.data,n.created_at FROM purchase_indent_dispatch_notices n JOIN purchase_indent_documents d ON d.id=n.document_id WHERE n.recipient_id= @recipient_id AND n.resolved_at IS NULL ORDER BY n.created_at DESC,n.id LIMIT 200;

-- name: IndentDocumentsLegacyTerms :many
SELECT l.document_id,l.terms FROM purchase_indent_legacy_terms l
JOIN purchase_indent_documents d ON d.id=l.document_id
WHERE d.project_id = @project_id AND d.bo_id = @bo_id;
