-- name: IdentityGuard :exec
SELECT pg_advisory_xact_lock_shared(hashtextextended('auth.administration',0));
-- name: OperationLock :exec
SELECT pg_advisory_xact_lock(hashtextextended(@operation_key::text,0));
-- name: FindOperation :one
SELECT * FROM document_package_operations WHERE actor_id= @actor_id AND command= @command AND context_key= @context_key AND idempotency_key= @idempotency_key;
-- name: SaveOperation :exec
INSERT INTO document_package_operations(actor_id,command,context_key,idempotency_key,fingerprint,receipt_id)VALUES(@actor_id,@command,@context_key,@idempotency_key,@fingerprint,@receipt_id);
-- name: CreateReceipt :exec
INSERT INTO document_package_receipts(id,owner_id,created_by_id,updated_by_id)VALUES(@id,@actor_id,@actor_id,@actor_id);
-- name: CreateFile :exec
INSERT INTO document_package_files(id,receipt_id,client_file_key,filename,kind,declared_size,declared_sha256,created_by_id,updated_by_id)VALUES(@id,@receipt_id,@client_file_key,@filename,@kind,@declared_size,@declared_sha256,@actor_id,@actor_id);
-- name: GetReceipt :one
SELECT * FROM document_package_receipts WHERE id= @id FOR UPDATE;
-- name: ListFiles :many
SELECT * FROM document_package_files WHERE receipt_id= @receipt_id ORDER BY created_at,id;
-- name: GetFile :one
SELECT * FROM document_package_files WHERE id= @id AND receipt_id= @receipt_id FOR UPDATE;
-- name: ActiveUploads :one
SELECT count(*) FROM document_package_files f JOIN document_package_receipts r ON r.id=f.receipt_id WHERE r.owner_id= @actor_id AND f.state='uploading' AND f.lease_until>clock_timestamp();
-- name: StartUpload :execrows
UPDATE document_package_files SET state='uploading',version=version+1,claim_id= @claim_id,object_key= @claim_id,lease_until=clock_timestamp()+interval '10 minutes',upload_key= @upload_key,upload_input_version=version,updated_at=clock_timestamp(),updated_by_id= @actor_id
WHERE id= @id AND version= @version AND (state='declared' OR (state='uploading' AND lease_until<=clock_timestamp()));
-- name: CompleteUpload :execrows
UPDATE document_package_files SET state='received',claim_id=NULL,lease_until=NULL,version=version+1,updated_at=clock_timestamp(),updated_by_id= @actor_id
WHERE id= @id AND state='uploading' AND claim_id= @claim_id AND upload_key= @upload_key AND lease_until>clock_timestamp();
-- name: TouchReceipt :exec
UPDATE document_package_receipts SET version=version+1,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE id= @id;
-- name: SealReceipt :execrows
UPDATE document_package_receipts SET state='verifying',version=version+1,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE id= @id AND version= @version AND state IN('receiving','failed') AND attempts<3 AND expires_at>clock_timestamp();
-- name: ClaimReceipt :one
WITH next AS (SELECT id FROM document_package_receipts WHERE state='verifying' AND attempts<3 AND expires_at>clock_timestamp() AND (lease_until IS NULL OR lease_until<clock_timestamp()) ORDER BY created_at,id FOR UPDATE SKIP LOCKED LIMIT 1)
UPDATE document_package_receipts r SET claim_id= @claim_id,lease_until=clock_timestamp()+interval '1 minute',attempts=least(attempts+1,3),updated_at=clock_timestamp(),updated_by_id=NULL FROM next n WHERE r.id=n.id RETURNING r.*;
-- name: AddMember :exec
INSERT INTO document_package_members(id,receipt_id,file_id,path,normalized_path,filename,kind,byte_size,expanded_bytes,sha256,reader_payload,reader_payload_sha256,validation_claim_id,scanner_revision,pages,sheets,cells,created_by_id)
VALUES(@id,@receipt_id,@file_id,@path,@normalized_path,@filename,@kind,@byte_size,@expanded_bytes,@sha256,@reader_payload,@reader_payload_sha256,@validation_claim_id,@scanner_revision,@pages,@sheets,@cells,@actor_id);
-- name: MemberTotals :one
SELECT count(*)::bigint AS members,COALESCE(sum(expanded_bytes),0)::bigint AS bytes,COALESCE(sum(pages),0)::bigint AS pages,COALESCE(sum(sheets),0)::bigint AS sheets,COALESCE(sum(cells),0)::bigint AS cells FROM document_package_members WHERE receipt_id= @receipt_id AND validation_claim_id= @claim_id;
-- name: FinishVerification :execrows
UPDATE document_package_receipts SET state= @state,error_code=sqlc.narg(error_code),lease_until=CASE WHEN @state::text='verifying' THEN clock_timestamp()+interval '30 seconds' ELSE NULL END,version=version+1,updated_at=clock_timestamp(),updated_by_id=owner_id
WHERE id= @id AND state='verifying' AND claim_id= @claim_id AND lease_until>clock_timestamp() AND expires_at>clock_timestamp();
-- name: ReadyFiles :exec
UPDATE document_package_files SET state='ready',version=version+1,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE receipt_id= @receipt_id AND state='received';
-- name: ConsumeReceipt :execrows
UPDATE document_package_receipts SET state='consumed',consumed_by_tender_id= @tender_id,lease_until=NULL,version=version+1,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE id= @id AND version= @version AND state='ready' AND draft_id IS NULL AND expires_at>clock_timestamp();
-- name: MemberIDs :many
SELECT m.id FROM document_package_members m JOIN document_package_receipts r ON r.id=m.receipt_id WHERE r.id= @receipt_id AND m.validation_claim_id= @claim_id AND r.state='consumed' AND (r.consumed_by_tender_id= @tender_id OR r.draft_id= @tender_id) ORDER BY m.normalized_path,m.id;
-- name: GetMember :one
SELECT m.* FROM document_package_members m JOIN document_package_receipts r ON r.id=m.receipt_id WHERE r.id= @receipt_id AND m.id= @id AND m.validation_claim_id= @claim_id AND r.state='consumed' AND (r.consumed_by_tender_id= @tender_id OR r.draft_id= @tender_id);
-- name: AddEvent :exec
INSERT INTO document_package_events(id,receipt_id,file_id,action,version,error_code,created_by_id)VALUES(@id,@receipt_id,sqlc.narg(file_id),@action,@version,sqlc.narg(error_code),sqlc.narg(actor_id));

-- name: FailExhaustedVerification :many
UPDATE document_package_receipts SET state='failed',error_code='verification_attempts_exhausted',lease_until=NULL,version=version+1,updated_at=clock_timestamp(),updated_by_id=owner_id WHERE state='verifying' AND attempts>=3 AND lease_until<clock_timestamp() RETURNING *;
-- name: AbortUpload :execrows
UPDATE document_package_files SET state='declared',claim_id=NULL,lease_until=NULL,version=version+1,error_code='upload_failed',updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE id= @id AND state='uploading' AND claim_id= @claim_id;

-- name: RecordUploadObject :exec
INSERT INTO document_package_objects(id,receipt_id,file_id)VALUES(@id,@receipt_id,@file_id);
-- name: ClaimPurgeObjects :many
WITH candidates AS (
 SELECT o.id FROM document_package_objects o JOIN document_package_receipts r ON r.id=o.receipt_id JOIN document_package_files f ON f.id=o.file_id
 WHERE o.purged_at IS NULL AND o.created_at<clock_timestamp()-interval '1 hour' AND (o.purge_lease_until IS NULL OR o.purge_lease_until<clock_timestamp())
 AND r.state<>'consumed' AND (r.lease_until IS NULL OR r.lease_until<clock_timestamp()) AND (f.lease_until IS NULL OR f.lease_until<clock_timestamp())
 AND (r.expires_at<clock_timestamp() OR f.object_key IS DISTINCT FROM o.id) ORDER BY o.created_at,o.id FOR UPDATE OF o SKIP LOCKED LIMIT 20
) UPDATE document_package_objects o SET purge_claim_id= @claim_id,purge_lease_until=clock_timestamp()+interval '10 minutes' FROM candidates c WHERE o.id=c.id RETURNING o.*;
-- name: FinishPurgeObject :execrows
UPDATE document_package_objects SET purged_at=clock_timestamp(),purge_lease_until=NULL WHERE id= @id AND purge_claim_id= @claim_id AND purge_lease_until>clock_timestamp() AND purged_at IS NULL;
-- name: ExpireReceipt :execrows
UPDATE document_package_receipts SET state='expired',error_code='receipt_expired',version=version+1,updated_at=clock_timestamp(),updated_by_id=owner_id WHERE id= @id AND state NOT IN('consumed','expired') AND expires_at<clock_timestamp();

-- name: RetainDraftReceipt :execrows
UPDATE document_package_receipts SET draft_id= @draft_id,expires_at='9999-12-31'::timestamptz,updated_at=clock_timestamp(),updated_by_id= @actor_id
WHERE id= @id AND owner_id= @actor_id AND state IN('receiving','verifying','ready','failed') AND expires_at>clock_timestamp() AND (draft_id IS NULL OR draft_id= @draft_id);
-- name: DraftReceipts :many
SELECT id FROM document_package_receipts WHERE draft_id= @draft_id ORDER BY created_at,id;
-- name: PublishDraftReceipt :execrows
UPDATE document_package_receipts SET state='consumed',published_at=clock_timestamp(),version=version+1,updated_at=clock_timestamp(),updated_by_id=owner_id WHERE id= @id AND draft_id= @draft_id AND state='ready';
-- name: ReadyDrafts :many
SELECT DISTINCT draft_id,owner_id FROM document_package_receipts WHERE draft_id IS NOT NULL AND state='ready' LIMIT 100;

-- name: DraftMemberInfo :one
SELECT m.id,m.receipt_id,m.file_id,m.path,m.filename,m.kind,m.sha256,m.pages,m.sheets,m.cells FROM document_package_members m JOIN document_package_receipts r ON r.id=m.receipt_id WHERE r.id= @receipt_id AND m.id= @id AND r.draft_id= @draft_id AND r.state='consumed' AND m.validation_claim_id=r.claim_id;

-- name: RenewVerificationClaim :execrows
UPDATE document_package_receipts SET lease_until=clock_timestamp()+interval '1 minute',updated_at=clock_timestamp() WHERE id= @id AND state='verifying' AND claim_id= @claim_id AND lease_until>clock_timestamp();

-- name: AddDisposition :exec
INSERT INTO document_package_manifest(id,receipt_id,file_id,claim_id,path,disposition,error_code) VALUES(@id,@receipt_id,@file_id,@claim_id,@path,@disposition,@error_code);
-- name: ListDispositions :many
SELECT * FROM document_package_manifest WHERE receipt_id= @receipt_id AND claim_id= @claim_id ORDER BY created_at,id;
