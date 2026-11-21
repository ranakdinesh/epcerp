ALTER TABLE public.communication_notices
    DROP CONSTRAINT communication_notices_notice_kind_check,
    ADD CONSTRAINT communication_notices_notice_kind_check
        CHECK (notice_kind IN ('director', 'quotation', 'supplier_review', 'tender_manager', 'procurement_manager', 'purchase_advance'));

CREATE OR REPLACE FUNCTION public.protect_communication_notice() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 IF TG_OP='INSERT' THEN
  IF NEW.version<>1 OR NEW.read_at IS NOT NULL OR NEW.acknowledged_at IS NOT NULL OR NEW.created_at<>NEW.updated_at OR NEW.created_by_id IS DISTINCT FROM NEW.updated_by_id THEN
    RAISE EXCEPTION 'New notice must be unread with source attribution';
  END IF;
  IF NEW.notice_kind='director' THEN
    IF NEW.project_id IS NULL THEN RAISE EXCEPTION 'Director notice requires project'; END IF;
    IF NOT EXISTS(SELECT 1 FROM business_events e WHERE e.id=NEW.source_event_id AND e.source_module IN('finance_approvals','finance_accounts_acceptance') AND e.entity_type='financial_decision' AND e.entity_id=NEW.decision_id AND e.entity_version=NEW.source_version AND e.action=NEW.decision_action AND e.actor_id=NEW.created_by_id AND e.actor_kind='user' AND e.scope_kind='project' AND e.scope_id=NEW.project_id AND e.visibility='restricted' AND e.occurred_at=NEW.created_at) THEN
      RAISE EXCEPTION 'Notice source must be its recorded financial decision';
    END IF;
    IF NOT EXISTS(SELECT 1 FROM organisation_director_designation d JOIN auth.users u ON u.id=d.user_id WHERE d.id=NEW.director_designation_id AND d.version=NEW.director_version AND d.user_id=NEW.recipient_id AND d.is_active AND u.is_active) THEN
      RAISE EXCEPTION 'Notice requires the effective Director';
    END IF;
  ELSIF NEW.notice_kind='tender_manager' THEN
    IF NEW.project_id IS NOT NULL OR NEW.director_designation_id IS NOT NULL OR NEW.director_version<>0 OR NEW.source_action_id IS NULL OR NEW.title='' OR NOT EXISTS(
      SELECT 1 FROM business_events e WHERE e.id=NEW.source_event_id AND e.source_module='tender_fms' AND e.entity_type='tender_assignment'
      AND e.entity_id=NEW.decision_id AND e.entity_version=NEW.source_version AND e.action='manager_assigned' AND NEW.decision_action='assigned'
      AND e.actor_id=NEW.created_by_id AND e.actor_kind='user' AND e.scope_kind='platform' AND e.scope_id IS NULL AND e.visibility='restricted' AND e.occurred_at=NEW.created_at
    ) THEN
      RAISE EXCEPTION 'Tender Manager notice requires its recorded assignment source';
    END IF;
  ELSIF NEW.notice_kind='supplier_review' THEN
    IF NEW.project_id IS NOT NULL OR NEW.director_designation_id IS NOT NULL OR NEW.director_version<>0 OR NEW.title='' OR NOT EXISTS(
      SELECT 1 FROM business_events e JOIN supplier_review_assignments a ON a.id=e.entity_id
      WHERE e.id=NEW.source_event_id AND e.source_module='supplier_review_assignments' AND e.entity_type='supplier_review_assignment'
      AND e.entity_id=NEW.decision_id AND e.entity_version=NEW.source_version AND e.action='assigned' AND NEW.decision_action='assigned'
      AND e.actor_id=NEW.created_by_id AND e.actor_kind='user' AND e.scope_kind='platform' AND e.scope_id IS NULL
      AND e.visibility='restricted' AND e.occurred_at=NEW.created_at AND a.version=NEW.source_version AND a.user_id=NEW.recipient_id AND a.is_active
      AND NEW.link='/dashboard?screen=suppliers&supplier_id='||a.supplier_id::text
    ) THEN
      RAISE EXCEPTION 'Supplier review notice requires current recorded assignment';
    END IF;
  ELSIF NEW.notice_kind='procurement_manager' THEN
    IF NEW.project_id IS NULL OR NEW.director_designation_id IS NOT NULL OR NEW.director_version<>0 OR NEW.title='' OR NEW.link !~ '^/dashboard\?screen=purchase-fms' OR NOT EXISTS(
      SELECT 1 FROM business_events e
      WHERE e.id=NEW.source_event_id AND e.source_module IN ('projects', 'purchase_fms')
      AND e.entity_id=NEW.decision_id AND e.entity_version=NEW.source_version
      AND e.actor_id=NEW.created_by_id AND e.actor_kind='user' AND e.scope_kind='project' AND e.scope_id=NEW.project_id
      AND e.visibility='restricted' AND e.occurred_at=NEW.created_at
    ) THEN
      RAISE EXCEPTION 'Procurement manager notice requires recorded project purchase source';
    END IF;
  ELSIF NEW.notice_kind='purchase_advance' THEN
    IF NEW.project_id IS NULL OR NEW.director_designation_id IS NOT NULL OR NEW.director_version<>0
      OR NEW.title='' OR NEW.link !~ '^/dashboard\?screen=purchase-advances'
      OR NEW.decision_action<>'submit' OR NOT EXISTS (
        SELECT 1 FROM business_events e JOIN purchase_advances a ON a.id=e.entity_id
        WHERE e.id=NEW.source_event_id AND e.source_module='purchase_advances'
          AND e.entity_type='purchase_advance' AND e.action='submit'
          AND e.entity_id=NEW.decision_id AND e.entity_version=NEW.source_version
          AND e.actor_id=NEW.created_by_id AND e.actor_kind='user'
          AND e.scope_kind='project' AND e.scope_id=NEW.project_id AND e.visibility='restricted'
          AND e.occurred_at=NEW.created_at AND a.project_id=NEW.project_id
          AND a.state='requested' AND a.version=NEW.source_version
      ) THEN
      RAISE EXCEPTION 'Advance notice requires its saved submitted request';
    END IF;
  ELSE
    IF NEW.project_id IS NULL THEN RAISE EXCEPTION 'Quotation notice requires project'; END IF;
    IF NEW.director_designation_id IS NOT NULL OR NEW.director_version<>0 OR NEW.title='' OR NEW.link !~ '^/dashboard\?' OR NOT EXISTS(
      SELECT 1 FROM business_events e WHERE e.id=NEW.source_event_id AND e.source_module='quotation_approvals' AND e.entity_type='quotation_approval_set'
      AND e.entity_id=NEW.decision_id AND e.entity_version=NEW.source_version AND e.action=NEW.decision_action AND e.action IN('submit','accounts_decision')
      AND e.actor_id=NEW.created_by_id AND e.scope_kind='project' AND e.scope_id=NEW.project_id AND e.visibility='restricted' AND e.occurred_at=NEW.created_at
    ) THEN
      RAISE EXCEPTION 'Addressed quotation notice requires its recorded source';
    END IF;
  END IF;
 ELSE
  IF ROW(NEW.source_action_id,NEW.notice_kind,NEW.title,NEW.link,NEW.id,NEW.source_event_id,NEW.recipient_id,NEW.director_designation_id,NEW.director_version,NEW.decision_id,NEW.project_id,NEW.source_version,NEW.decision_action,NEW.is_active,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.source_action_id,OLD.notice_kind,OLD.title,OLD.link,OLD.id,OLD.source_event_id,OLD.recipient_id,OLD.director_designation_id,OLD.director_version,OLD.decision_id,OLD.project_id,OLD.source_version,OLD.decision_action,OLD.is_active,OLD.created_at,OLD.created_by_id) THEN
    RAISE EXCEPTION 'Notice identity and addressed source are immutable';
  END IF;
  IF NEW.version<>OLD.version+1 OR NEW.updated_by_id IS DISTINCT FROM NEW.recipient_id OR NEW.updated_at<OLD.updated_at OR (OLD.read_at IS NOT NULL AND NEW.read_at IS DISTINCT FROM OLD.read_at) OR (OLD.acknowledged_at IS NOT NULL AND NEW.acknowledged_at IS DISTINCT FROM OLD.acknowledged_at) OR ROW(NEW.read_at,NEW.acknowledged_at) IS NOT DISTINCT FROM ROW(OLD.read_at,OLD.acknowledged_at) THEN
    RAISE EXCEPTION 'Notice receipts must advance monotonically for the recipient';
  END IF;
 END IF;
 RETURN NEW;
END $$;

-- Only conditional client approvals require evidence that their conditions are closed.
CREATE OR REPLACE FUNCTION public.protect_purchase_inspection() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE o purchase_orders;a purchase_acceptances;t purchase_technical_cases;expected jsonb;q jsonb;i jsonb;point jsonb;codes text[]='{}';day date;BEGIN
 SELECT * INTO o FROM purchase_orders WHERE id=NEW.order_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=NEW.order_version AND state='released' AND is_active;
 SELECT * INTO a FROM purchase_acceptances WHERE id=NEW.acceptance_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND order_id=NEW.order_id AND order_version=NEW.order_version AND version=NEW.acceptance_version AND status='accepted' AND is_active;
 SELECT * INTO t FROM purchase_technical_cases WHERE id=NEW.technical_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=NEW.technical_version AND state IN('approved','approved_with_comments') AND is_active;
 IF o.id IS NULL OR a.id IS NULL OR t.id IS NULL THEN RAISE EXCEPTION 'Inspection requires its current released PO, unconditional acceptance and approved technical kit';END IF;
 SELECT d INTO q FROM jsonb_array_elements(t.data->'documents') d WHERE d->>'kind'='qap' AND d->>'status'='received' AND d->>'is_active'='true';
 IF q IS NULL OR t.data->'approval'->>'qap_approved' IS DISTINCT FROM 'true' OR (t.state='approved_with_comments' AND t.data->'approval'->>'conditions_closed' IS DISTINCT FROM 'true') OR NOT EXISTS(SELECT 1 FROM purchase_technical_files WHERE id=(q->>'file_id')::uuid AND case_id=t.id AND purpose='document' AND is_active) OR NOT EXISTS(SELECT 1 FROM purchase_technical_files WHERE id=(t.data->'approval'->>'file_id')::uuid AND case_id=t.id AND purpose='client_approval' AND is_active) THEN RAISE EXCEPTION 'Inspection requires actual approved QAP and signed client approval files';END IF;
 expected=jsonb_build_object('project_id',NEW.project_id,'package_id',NEW.package_id,'order_id',o.id,'order_version',o.version,'number',o.number,'date',o.data->'details'->>'date','supplier_id',o.source_snapshot->'supplier'->>'id','supplier_name',o.source_snapshot->'supplier'->>'name','description',o.source_snapshot->>'description','quantity',o.source_snapshot->>'quantity','uom',o.source_snapshot->>'uom','acceptance_id',a.id,'acceptance_version',a.version,'technical_id',t.id,'technical_version',t.version,'technical_revision',t.revision,'qap_reference',t.data->'approval'->>'reference','qap_date',t.data->'approval'->>'date','qap_file_id',q->>'file_id','signed_approval_file_id',t.data->'approval'->>'file_id','available',true,'blockers','[]'::jsonb);
 IF NEW.source_snapshot IS DISTINCT FROM expected THEN RAISE EXCEPTION 'Inspection source must equal its current approved evidence';END IF;
 IF NEW.data->>'id' IS DISTINCT FROM NEW.id::text OR NEW.data->>'project_id' IS DISTINCT FROM NEW.project_id::text OR NEW.data->>'package_id' IS DISTINCT FROM NEW.package_id::text OR NEW.data->>'version' IS DISTINCT FROM NEW.version::text OR NEW.data->>'revision' IS DISTINCT FROM NEW.revision::text OR NEW.data->>'is_active' IS DISTINCT FROM NEW.is_active::text OR NEW.data->'source_snapshot' IS DISTINCT FROM NEW.source_snapshot OR NEW.data->'inspection'->>'status' IS DISTINCT FROM NEW.status THEN RAISE EXCEPTION 'Inspection projection mismatch';END IF;
 i=NEW.data->'inspection';
 IF jsonb_typeof(i->'witness_points') IS DISTINCT FROM 'array' OR jsonb_array_length(i->'witness_points')>100 THEN RAISE EXCEPTION 'Inspection has a bounded witness checklist';END IF;
 IF coalesce(i->>'inspection_agency','') NOT IN('','client_consultant','tpi','internal_qc') THEN RAISE EXCEPTION 'Choose a declared inspection agency';END IF;
 IF NEW.status='call_received' AND (length(trim(coalesce(i->>'call_notice_ref','')))=0 OR coalesce(i->>'call_date','')='') THEN RAISE EXCEPTION 'Vendor inspection call needs reference and actual date';END IF;
 IF NEW.status IN('scheduled','cleared','rejected') AND length(trim(coalesce(i->>'factory_location','')))=0 THEN RAISE EXCEPTION 'Inspection requires the actual factory';END IF;
 IF NEW.status='scheduled' AND coalesce(i->>'scheduled_date','')='' AND coalesce(i->>'actual_date','')='' THEN RAISE EXCEPTION 'Schedule an inspection date';END IF;
 IF NEW.status IN('cleared','rejected') AND (coalesce(i->>'actual_date','')='' OR length(trim(coalesce(i->>'inspector_name','')))=0 OR length(trim(coalesce(i->>'findings','')))=0 OR coalesce(i->>'inspection_agency','')='') THEN RAISE EXCEPTION 'Final inspection requires actual visit, inspector, agency and findings';END IF;
 FOR point IN SELECT value FROM jsonb_array_elements(i->'witness_points') LOOP
  IF jsonb_typeof(point) IS DISTINCT FROM 'object' OR length(trim(coalesce(point->>'code','')))=0 OR length(point->>'code')>60 OR length(trim(coalesce(point->>'title','')))=0 OR length(point->>'title')>1000 OR length(coalesce(point->>'remarks',''))>2000 OR jsonb_typeof(point->'is_witness') IS DISTINCT FROM 'boolean' OR jsonb_typeof(point->'is_cleared') IS DISTINCT FROM 'boolean' THEN RAISE EXCEPTION 'Witness points require explicit valid code, title and flags';END IF;
  IF lower(point->>'code')=ANY(codes) THEN RAISE EXCEPTION 'Witness point codes must be unique';END IF;
  codes=array_append(codes,lower(point->>'code'));
  IF NEW.status='cleared' AND point->>'is_witness'='true' AND point->>'is_cleared'<>'true' THEN RAISE EXCEPTION 'Clear every mandatory QAP witness point';END IF;
 END LOOP;
 IF NEW.status='cleared' AND jsonb_array_length(i->'witness_points')=0 AND length(trim(coalesce(i->>'no_witness_points_reason','')))=0 THEN RAISE EXCEPTION 'An empty approved QAP checklist requires a clearance reason';END IF;
 IF jsonb_array_length(i->'witness_points')>0 AND coalesce(i->>'no_witness_points_reason','')<>'' THEN RAISE EXCEPTION 'No-points reason applies only to empty checklists';END IF;
 FOR q IN SELECT to_jsonb(x) FROM unnest(ARRAY['call_date','scheduled_date','actual_date']) x LOOP
  IF coalesce(i->>(q#>>'{}'),'')<>'' THEN
   day=(i->>(q#>>'{}'))::date;
   IF day<(o.data->'details'->>'date')::date OR (q#>>'{}')<>'scheduled_date' AND day>(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date OR (q#>>'{}')<>'call_date' AND coalesce(i->>'call_date','')<>'' AND day<(i->>'call_date')::date OR (q#>>'{}')='actual_date' AND day<(t.data->'approval'->>'date')::date THEN RAISE EXCEPTION 'Inspection dates must reflect actual approved evidence';END IF;
  END IF;
 END LOOP;
 IF TG_OP='INSERT' AND (NEW.version<>1 OR NEW.revision<>1) THEN RAISE EXCEPTION 'Inspection begins at version and revision one';END IF;
 IF TG_OP='UPDATE' THEN
  IF NEW.id<>OLD.id OR NEW.project_id<>OLD.project_id OR NEW.package_id<>OLD.package_id OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'Inspection ownership and audit must be retained';END IF;
  IF NEW.revision=OLD.revision THEN
   IF OLD.status IN('cleared','rejected') OR NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot THEN RAISE EXCEPTION 'Final or stale inspection requires an explicit revision';END IF;
  ELSIF NEW.revision<>OLD.revision+1 OR NEW.status<>'not_scheduled' OR i IS DISTINCT FROM jsonb_build_object('status','not_scheduled','call_notice_ref','','call_date','','scheduled_date','','actual_date','','inspection_agency','','inspector_name','','factory_location','','witness_points','[]'::jsonb,'no_witness_points_reason','','findings','') THEN RAISE EXCEPTION 'Revision must reset current results and retain old evidence';END IF;
 END IF;
 RETURN NEW;END $$;
