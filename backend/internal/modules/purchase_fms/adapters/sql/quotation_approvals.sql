-- name: QuotationApprovalsGetRecord :one
SELECT * FROM quotation_approval_sets WHERE project_id= @project_id AND bo_id= @bo_id;
-- name: QuotationApprovalsInsertRecord :one
INSERT INTO quotation_approval_sets(project_id,bo_id,version,attempts,is_active,created_by_id,updated_by_id)VALUES(@project_id,@bo_id,1,@attempts,true,@actor_id,@actor_id)RETURNING *;
-- name: QuotationApprovalsUpdateRecord :one
UPDATE quotation_approval_sets SET attempts= @attempts,version=version+1,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE project_id= @project_id AND bo_id= @bo_id AND version= @expected_version RETURNING *;
-- name: QuotationApprovalsEvent :exec
INSERT INTO quotation_approval_events(id,bo_id,action,reason,before_snapshot,after_snapshot,created_at,created_by_id)VALUES(@id,@bo_id,@action,@reason,@before_snapshot,@after_snapshot,@created_at,@actor_id);
-- name: QuotationApprovalsHistory :many
SELECT e.* FROM quotation_approval_events e JOIN quotation_approval_sets q ON q.bo_id=e.bo_id WHERE q.project_id= @project_id AND q.bo_id= @bo_id ORDER BY e.created_at DESC,e.id LIMIT 200;
-- name: QuotationApprovalsQueueRecords :many
SELECT project_id,bo_id,version,updated_at FROM quotation_approval_sets WHERE is_active ORDER BY bo_id LIMIT 50 OFFSET @record_offset;
-- name: QuotationWorkCreate :exec
INSERT INTO purchase_quotation_work_tasks(id,project_id,bo_id,group_id,attempt_id,recipient_id,kind,title,link,state,created_by_id,updated_by_id)
VALUES(@id,@project_id,@bo_id,@group_id,@attempt_id,@recipient_id,@kind,@title,@link,@state,@actor_id,@actor_id);
-- name: QuotationWorkResolve :exec
UPDATE purchase_quotation_work_tasks SET state='completed',updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE attempt_id= @attempt_id AND state<>'completed';
-- name: QuotationWorkOwn :many
SELECT * FROM purchase_quotation_work_tasks WHERE recipient_id= @recipient_id AND state<>'completed' ORDER BY created_at DESC,id LIMIT 100;

-- name: QuotationWorkReturn :exec
UPDATE purchase_quotation_work_tasks SET state= @state,title= @title,link= @link,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE attempt_id= @attempt_id AND kind='executive';

-- name: QuotationPendingDocument :one
SELECT EXISTS(SELECT 1 FROM quotation_approval_sets q CROSS JOIN LATERAL jsonb_array_elements(q.attempts) a JOIN purchase_indent_documents d ON d.group_id::text=a->>'group_id' AND d.id= @document_id WHERE q.bo_id= @bo_id AND q.project_id= @project_id AND a->>'state'='pending_accounts' AND jsonb_array_length(coalesce(a->'proposals','[]'))>0 AND a=(SELECT value FROM jsonb_array_elements(q.attempts) WITH ORDINALITY x(value,n) WHERE value->>'group_id'=a->>'group_id' ORDER BY n DESC LIMIT 1));
-- name: QuotationProjectRecords :many
SELECT * FROM quotation_approval_sets WHERE project_id= @project_id AND is_active ORDER BY bo_id;
-- name: QuotationWorkResolveEarlier :exec
UPDATE purchase_quotation_work_tasks SET state='completed',updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE bo_id= @bo_id AND group_id= @group_id AND state<>'completed';
-- name: QuotationPendingGroups :many
SELECT DISTINCT (a->>'group_id')::uuid AS group_id FROM quotation_approval_sets q CROSS JOIN LATERAL jsonb_array_elements(q.attempts) WITH ORDINALITY AS x(a,ord)
WHERE q.bo_id= @bo_id AND q.project_id= @project_id AND a->>'state'='pending_accounts' AND jsonb_array_length(coalesce(a->'proposals','[]'))>0
AND NOT EXISTS(SELECT 1 FROM jsonb_array_elements(q.attempts) WITH ORDINALITY AS y(later,n) WHERE n>ord AND later->>'group_id'=a->>'group_id');

-- name: QuotationDraftList :many
SELECT * FROM purchase_quotation_shortlist_drafts WHERE project_id= @project_id AND bo_id= @bo_id ORDER BY group_id;
-- name: QuotationDraftInsert :one
INSERT INTO purchase_quotation_shortlist_drafts(project_id,bo_id,group_id,version,proposals,comment,created_by_id,updated_by_id)
VALUES(@project_id,@bo_id,@group_id,1,@proposals,@comment,@actor_id,@actor_id) ON CONFLICT DO NOTHING RETURNING *;
-- name: QuotationDraftUpdate :one
UPDATE purchase_quotation_shortlist_drafts SET version=version+1,proposals= @proposals,comment= @comment,is_active=true,updated_at=clock_timestamp(),updated_by_id= @actor_id
WHERE project_id= @project_id AND bo_id= @bo_id AND group_id= @group_id AND version= @expected_version RETURNING *;
-- name: QuotationDraftComplete :many
UPDATE purchase_quotation_shortlist_drafts SET version=version+1,is_active=false,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE project_id= @project_id AND bo_id= @bo_id AND group_id= @group_id AND is_active RETURNING *;
-- name: QuotationDraftHistory :exec
INSERT INTO purchase_quotation_shortlist_history(id,project_id,bo_id,group_id,version,action,snapshot,created_by_id)VALUES(@id,@project_id,@bo_id,@group_id,@version,@action,@snapshot,@actor_id);
