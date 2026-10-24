-- name: LockIntakeUploads :exec
SELECT pg_advisory_xact_lock(91827147);
-- name: ListIntakes :many
SELECT * FROM tender_fms_intakes WHERE record_kind='legacy' AND (@owner::text='' OR owner_id::text= @owner::text) ORDER BY created_at DESC,id;
-- name: GetIntake :one
SELECT * FROM tender_fms_intakes WHERE id= @id;
-- name: FindIntake :one
SELECT * FROM tender_fms_intakes WHERE record_kind='legacy' AND owner_id= @owner_id AND initial_sha256= @sha256;
-- name: InsertIntake :one
INSERT INTO tender_fms_intakes(id,title,legacy_title,owner_id,initial_sha256,created_by_id,updated_by_id)VALUES(@id,@title::text,@title::text,@actor_id,@sha256,@actor_id,@actor_id) RETURNING *;
-- name: UpdateIntake :one
UPDATE tender_fms_intakes SET version=version+1,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE id= @id RETURNING *;
-- name: InsertIntakeEvent :exec
INSERT INTO tender_fms_events(id,intake_id,document_id,action,actor_id)VALUES(@id,@intake_id,@document_id,@action,@actor_id);
-- name: ExistingAttachment :one
SELECT EXISTS(SELECT 1 FROM tender_fms_events WHERE intake_id= @intake_id AND document_id= @document_id AND action='source_attached');
-- name: IntakeHistory :many
SELECT e.id,e.document_id,e.action,e.actor_id,e.created_at,0::integer AS version,''::text AS reason FROM tender_fms_events e WHERE e.intake_id= @subject_id
UNION ALL
SELECT h.id,NULL::uuid AS document_id,'owner_handover'::varchar(40) AS action,h.created_by_id AS actor_id,h.created_at,h.version,h.reason FROM tender_fms_owner_changes h WHERE h.intake_id= @subject_id ORDER BY created_at DESC,id;

-- name: ReadWorkInventory :many
WITH assignments AS (
SELECT 'tender_owner'::text AS kind,p.id::text AS id,p.owner_id::text AS owner_id,CASE WHEN p.record_kind='named' THEN p.assignment_version ELSE p.version::bigint END AS version,p.title::text AS title,'active'::text AS state,'/dashboard?screen=tender-fms'::text AS link,CASE WHEN p.record_kind='named' THEN jsonb_build_array('tenders.register.view','tenders.register.read','tenders.register.write','tenders.processing.read','tenders.processing.write','tenders.register.assign','tenders.evaluation.read','tenders.evaluation.write','communication.inbox.read','communication.inbox.write') ELSE jsonb_build_array('tenders.register.view','tenders.register.read','tenders.register.write','tenders.processing.read','tenders.processing.write') END::jsonb AS required_permissions,''::text AS due_date FROM tender_fms_intakes p WHERE p.is_active
) SELECT jsonb_build_object('Source','tender_fms','Kind',kind,'ID',id,'OwnerID',owner_id,'Version',version,'Title',title,'State',state,'Link',link,'RequiredPermissions',required_permissions,'DueDate',due_date)::jsonb AS payload FROM assignments WHERE (sqlc.arg(owner_id)::text='' OR owner_id=sqlc.arg(owner_id)) AND (sqlc.arg(record_id)::text='' OR id=sqlc.arg(record_id)) AND (sqlc.arg(kind)::text='' OR kind=sqlc.arg(kind)) ORDER BY kind,id,owner_id LIMIT 10001;

-- name: HandoverTenderOwner :one
UPDATE tender_fms_intakes SET owner_id=$2,version=version+1,updated_at=clock_timestamp(),updated_by_id=$3 WHERE id=$1 AND version=$4 AND is_active RETURNING *;
-- name: RecordTenderOwnerChange :exec
INSERT INTO tender_fms_owner_changes(id,intake_id,previous_owner_id,owner_id,version,reason,created_by_id,created_at)VALUES($1,$2,$3,$4,$5,$6,$7,$8);

-- name: LockTenderIdentity :exec
SELECT pg_advisory_xact_lock_shared(hashtextextended('auth.administration',0));
-- name: InsertNamedTender :one
INSERT INTO tender_fms_intakes(id,title,owner_id,record_kind,sales_creator_id,manager_assignment_id,created_by_id,updated_by_id)
 VALUES(@id,@title,@manager_id,'named',@actor_id,@assignment_id,@actor_id,@actor_id) RETURNING *;
-- name: InsertNamedManagerAssignment :exec
INSERT INTO tender_fms_manager_assignments(id,intake_id,manager_id,version,reason,created_by_id)
 VALUES(@id,@intake_id,@manager_id,1,'Initial named Tender Manager assignment',@actor_id);
-- name: InsertNamedRecordEvent :exec
INSERT INTO tender_fms_record_events(id,intake_id,business_version,action,actor_id)
 VALUES(@id,@intake_id,1,'named_record_created',@actor_id);

-- name: NamedParticipantActive :one
SELECT COALESCE((SELECT is_active AND (valid_until IS NULL OR valid_until>clock_timestamp()) FROM tender_fms_participant_grants WHERE intake_id= @intake_id AND user_id= @user_id ORDER BY version DESC LIMIT 1),false)::boolean AS active;
-- name: LockNamedTender :one
SELECT * FROM tender_fms_intakes WHERE id= @id FOR UPDATE;
-- name: CurrentNamedGrantVersion :one
SELECT COALESCE(max(version),0)::bigint AS version FROM tender_fms_participant_grants WHERE intake_id= @intake_id AND user_id= @user_id;
-- name: RecordNamedGrant :one
INSERT INTO tender_fms_participant_grants(id,intake_id,user_id,version,is_active,valid_until,reason,created_by_id)
 VALUES(@id,@intake_id,@user_id,@version,@is_active,sqlc.narg(valid_until),@reason,@actor_id) RETURNING created_at;

-- name: CaptureOperationLock :exec
SELECT pg_advisory_xact_lock(hashtextextended(@operation_key::text,0));
-- name: FindCaptureOperation :one
SELECT * FROM tender_capture_operations WHERE actor_id= @actor_id AND command= @command AND context_key= @context_key AND idempotency_key= @idempotency_key;
-- name: SaveCaptureOperation :exec
INSERT INTO tender_capture_operations(actor_id,command,context_key,idempotency_key,fingerprint,tender_id)VALUES(@actor_id,@command,@context_key,@idempotency_key,@fingerprint,@tender_id);
-- name: BindCapture :exec
UPDATE tender_fms_intakes SET receipt_id= @receipt_id,external_reference=sqlc.narg(external_reference),source_url=sqlc.narg(source_url),updated_by_id= @actor_id WHERE id= @id;
-- name: CreateProvisionOperation :exec
INSERT INTO tender_provision_operations(id,tender_id,created_by_id,updated_by_id)VALUES(@id,@tender_id,@actor_id,@actor_id);
-- name: LinkTenderSource :exec
INSERT INTO tender_source_links(member_id,tender_id,document_id,created_by_id)VALUES(@member_id,@tender_id,@document_id,@actor_id);
-- name: CaptureProvision :one
SELECT * FROM tender_provision_operations WHERE tender_id= @tender_id;
-- name: CaptureDeliveries :many
SELECT a.id,a.notice_id,j.id AS email_job_id,j.state AS email_state FROM tender_source_actions a JOIN tender_email_jobs j ON j.action_id=a.id WHERE a.tender_id= @tender_id AND a.state='pending' ORDER BY a.created_at DESC LIMIT 10;
-- name: CreateManagerAction :one
INSERT INTO tender_source_actions(id,tender_id,assignment_id,recipient_id,source_event_id,assignment_version,created_at,updated_at,created_by_id,updated_by_id)
 VALUES(@id,@tender_id,@assignment_id,@recipient_id,@source_event_id,@assignment_version,clock_timestamp(),clock_timestamp(),@actor_id,@actor_id) RETURNING *;
-- name: CreateManagerEmail :exec
INSERT INTO tender_email_jobs(id,action_id,recipient_id,created_by_id,updated_by_id)VALUES(@id,@action_id,@recipient_id,@actor_id,@actor_id);
-- name: LinkManagerNotice :exec
UPDATE tender_source_actions SET notice_id= @notice_id WHERE id= @id;
-- name: CancelOldManagerWork :exec
UPDATE tender_source_actions SET state='cancelled_obsolete',is_active=false,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE tender_id= @tender_id AND state='pending';
-- name: CancelOldManagerMail :exec
UPDATE tender_email_jobs SET state='cancelled_obsolete',version=version+1,claim_id=NULL,lease_until=NULL,updated_at=clock_timestamp(),updated_by_id= @actor_id
WHERE action_id IN(SELECT id FROM tender_source_actions WHERE tender_id= @tender_id) AND state IN('queued','attempting');
-- name: InsertManagerAssignment :exec
INSERT INTO tender_fms_manager_assignments(id,intake_id,manager_id,version,reason,created_by_id)VALUES(@id,@intake_id,@manager_id,@version,@reason,@actor_id);
-- name: ReplaceNamedManager :one
UPDATE tender_fms_intakes SET owner_id= @manager_id,manager_assignment_id= @assignment_id,assignment_version=assignment_version+1,business_version=business_version+1,version=version+1,updated_at=clock_timestamp(),updated_by_id= @actor_id
 WHERE id= @id AND business_version= @expected_version AND assignment_version= @expected_assignment_version AND record_kind='named' AND is_active RETURNING *;
-- name: PatchNamedTender :one
UPDATE tender_fms_intakes SET title= @name,external_reference=CASE WHEN @set_reference::boolean THEN sqlc.narg(external_reference)::text ELSE external_reference END,source_url=CASE WHEN @set_url::boolean THEN sqlc.narg(source_url)::text ELSE source_url END,business_version=business_version+1,version=version+1,updated_at=clock_timestamp(),updated_by_id= @actor_id
WHERE id= @id AND business_version= @expected_version AND record_kind='named' AND is_active RETURNING *;
-- name: RecordNamedChange :exec
INSERT INTO tender_fms_record_events(id,intake_id,business_version,action,actor_id)VALUES(@id,@intake_id,@business_version,@action,@actor_id);
-- name: ListNamedCaptures :many
SELECT i.id FROM tender_fms_intakes i WHERE i.record_kind='named' AND i.receipt_id IS NOT NULL AND (@search::text='' OR position(lower(@search::text) in lower(i.title))>0) AND (@portfolio::boolean OR i.sales_creator_id= @actor_id OR i.owner_id= @actor_id OR COALESCE((SELECT g.is_active AND (g.valid_until IS NULL OR g.valid_until>clock_timestamp()) FROM tender_fms_participant_grants g WHERE g.intake_id=i.id AND g.user_id= @actor_id ORDER BY version DESC LIMIT 1),false)) ORDER BY CASE WHEN @sort_key::text='name' AND NOT @descending::boolean THEN i.title END ASC,CASE WHEN @sort_key::text='name' AND @descending::boolean THEN i.title END DESC,CASE WHEN @sort_key::text='created' AND NOT @descending::boolean THEN i.created_at END ASC,i.created_at DESC,i.id LIMIT @page_size::integer OFFSET @page_offset::bigint;
-- name: CountNamedCaptures :one
SELECT count(*) FROM tender_fms_intakes i WHERE i.record_kind='named' AND i.receipt_id IS NOT NULL AND (@search::text='' OR position(lower(@search::text) in lower(i.title))>0) AND (@portfolio::boolean OR i.sales_creator_id= @actor_id OR i.owner_id= @actor_id OR COALESCE((SELECT g.is_active AND (g.valid_until IS NULL OR g.valid_until>clock_timestamp()) FROM tender_fms_participant_grants g WHERE g.intake_id=i.id AND g.user_id= @actor_id ORDER BY version DESC LIMIT 1),false));
-- name: ReadAssignmentAction :one
SELECT * FROM tender_source_actions WHERE id= @id;
