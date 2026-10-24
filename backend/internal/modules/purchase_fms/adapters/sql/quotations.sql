-- name: QuotationsGetRecord :one
SELECT * FROM purchase_group_quotations WHERE project_id= @project_id AND bo_id= @bo_id;
-- name: QuotationsInsertRecord :one
INSERT INTO purchase_group_quotations(bo_id,project_id,version,quotes,is_active,created_by_id,updated_by_id)VALUES(@bo_id,@project_id,1,@quotes,true,@actor_id,@actor_id)RETURNING *;
-- name: QuotationsUpdateRecord :one
UPDATE purchase_group_quotations SET version=version+1,quotes= @quotes,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE project_id= @project_id AND bo_id= @bo_id AND version= @expected_version RETURNING *;
-- name: QuotationsInsertRevision :exec
INSERT INTO purchase_quotation_revisions(document_id,revision,bo_id,attachment_id,quote,created_by_id)VALUES(@document_id,@revision,@bo_id,sqlc.narg(attachment_id),@quote,@actor_id);
-- name: QuotationsRevisions :many
SELECT r.quote FROM purchase_quotation_revisions r JOIN purchase_group_quotations q ON q.bo_id=r.bo_id WHERE q.project_id= @project_id AND q.bo_id= @bo_id ORDER BY r.created_at,r.document_id,r.revision;
-- name: QuotationsInsertEvent :exec
INSERT INTO purchase_quotation_events(id,bo_id,action,reason,before_snapshot,after_snapshot,created_by_id)VALUES(@id,@bo_id,@action,@reason,@before_snapshot,@after_snapshot,@actor_id);
-- name: QuotationsHistory :many
SELECT e.* FROM purchase_quotation_events e JOIN purchase_group_quotations q ON q.bo_id=e.bo_id WHERE q.project_id= @project_id AND q.bo_id= @bo_id ORDER BY e.created_at DESC,e.id LIMIT 200;
-- name: QuotationsAttachments :many
SELECT id,document_id,filename,content_type,byte_size,sha256,is_active,created_at,created_by_id FROM purchase_quotation_attachments WHERE document_id= @document_id ORDER BY created_at,id;
-- name: QuotationsAttachment :one
SELECT * FROM purchase_quotation_attachments WHERE document_id= @document_id AND id= @id;
-- name: QuotationsInsertAttachment :one
INSERT INTO purchase_quotation_attachments(id,document_id,filename,content_type,byte_size,sha256,ciphertext,is_active,created_by_id,updated_by_id)VALUES(@id,@document_id,@filename,@content_type,@byte_size,@sha256,@ciphertext,true,@actor_id,@actor_id)RETURNING *;
-- name: QuotationsEvidenceEvent :exec
INSERT INTO purchase_quotation_evidence_events(id,attachment_id,action,created_by_id)VALUES(@id,@attachment_id,@action,@actor_id);
-- name: QuotationsEvidenceScope :one
SELECT project_id,bo_id FROM purchase_indent_documents WHERE id= @document_id;
