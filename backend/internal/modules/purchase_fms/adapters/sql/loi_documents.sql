-- name: LOIDocumentsList :many
SELECT * FROM purchase_loi_documents WHERE project_id= @project_id AND attempt_id= @attempt_id ORDER BY version DESC;
-- name: LOIDocumentsGet :one
SELECT * FROM purchase_loi_documents WHERE project_id= @project_id AND attempt_id= @attempt_id AND id= @id;
-- name: LOIDocumentsSave :exec
INSERT INTO purchase_loi_documents(id,project_id,handoff_id,attempt_id,version,reference,document_date,format,request_hash,content,sha256,object_bucket,object_key,byte_size,is_active,created_at,updated_at,created_by_id,updated_by_id)
VALUES(@id,@project_id,@handoff_id,@attempt_id,@version,@reference,@document_date,@format,@request_hash,@content,@sha256,@object_bucket,@object_key,@byte_size,true,@at,@at,@actor_id,@actor_id);
-- name: LOIDocumentsEvent :exec
INSERT INTO purchase_loi_document_events(id,document_id,action,created_at,created_by_id)VALUES(@id,@document_id,@action,@at,@actor_id);

-- name: LOIReviewGet :one
SELECT r.id,r.document_id,r.details_correct,r.prices_correct,r.format_correct,r.created_at,r.created_by_id,u.username AS reviewer
FROM purchase_loi_reviews r JOIN auth.users u ON u.id=r.created_by_id WHERE r.document_id= @document_id;

-- name: LOIReviewSave :exec
INSERT INTO purchase_loi_reviews(id,document_id,details_correct,prices_correct,format_correct,created_at,updated_at,created_by_id,updated_by_id)
VALUES(@id,@document_id,@details_correct,@prices_correct,@format_correct,@at,@at,@actor_id,@actor_id);
