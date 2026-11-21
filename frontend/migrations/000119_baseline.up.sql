-- EPCERP schema and required seed data at version 119.
-- Generated with PostgreSQL 17 from a clean database using the previous migrations.
-- The migrator manages public.schema_migrations separately.
BEGIN;

SET LOCAL check_function_bodies = false;
SELECT pg_catalog.set_config('search_path', '', true);

CREATE SCHEMA auth;

CREATE SCHEMA tender;

CREATE FUNCTION auth.advance_assignment_version() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF TG_TABLE_NAME='user_roles' THEN UPDATE auth.users SET edit_version=edit_version WHERE id=COALESCE(NEW.user_id,OLD.user_id);
 ELSE UPDATE auth.roles SET edit_version=edit_version WHERE id=COALESCE(NEW.role_id,OLD.role_id); END IF; RETURN NULL;
END $$;

CREATE FUNCTION auth.advance_edit_version() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN NEW.edit_version:=OLD.edit_version+1; RETURN NEW; END $$;

CREATE FUNCTION auth.audit_administration() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE before_value jsonb; after_value jsonb; subject uuid; actor uuid;
BEGIN
 actor:=NULLIF(current_setting('epcerp.auth_actor',true),'')::uuid;
 IF TG_OP<>'INSERT' THEN before_value:=to_jsonb(OLD)-ARRAY['password_hash','credential_version','temporary_password_expires_at']; END IF;
 IF TG_OP<>'DELETE' THEN after_value:=to_jsonb(NEW)-ARRAY['password_hash','credential_version','temporary_password_expires_at']; END IF;
 IF TG_TABLE_NAME IN ('users','roles') THEN
  subject:=COALESCE(after_value->>'id',before_value->>'id')::uuid;
  IF TG_OP='UPDATE' AND (before_value-ARRAY['updated_at','updated_by_id','edit_version','account_version']) IS NOT DISTINCT FROM (after_value-ARRAY['updated_at','updated_by_id','edit_version','account_version']) THEN RETURN NULL; END IF;
 ELSE subject:=COALESCE(after_value->>'user_id',before_value->>'user_id',after_value->>'role_id',before_value->>'role_id')::uuid;
 END IF;
 INSERT INTO auth.administration_changes(entity_type,entity_id,action,actor_id,before_snapshot,after_snapshot)
 VALUES(TG_TABLE_NAME,subject,lower(TG_OP),actor,before_value,after_value);
 RETURN NULL;
END $$;

CREATE FUNCTION auth.guard_auth_reporting_line() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(hashtextextended('auth.administration',0));
 IF NEW.is_active AND EXISTS (
  WITH RECURSIVE chain(id) AS (
   SELECT NEW.manager_id WHERE NEW.manager_id IS NOT NULL
   UNION SELECT r.manager_id FROM auth.auth_reporting_lines r JOIN chain c ON r.user_id=c.id WHERE r.is_active AND r.manager_id IS NOT NULL
  ) SELECT 1 FROM chain WHERE id=NEW.user_id
 ) THEN RAISE EXCEPTION 'Circular reporting line' USING ERRCODE='23514'; END IF;
 RETURN NEW;
END; $$;

CREATE FUNCTION auth.reject_auth_credential_change_mutation() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN RAISE EXCEPTION 'Credential history is immutable'; END; $$;

CREATE FUNCTION auth.reject_auth_organisation_change_mutation() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN RAISE EXCEPTION 'Organisation history is immutable'; END; $$;

CREATE FUNCTION public.check_award_handoff() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_awards' AND entity_id=NEW.id AND action='created' AND actor_id=NEW.created_by_id AND scope_id=NEW.project_id AND visibility='restricted') THEN RAISE EXCEPTION 'Award handoff requires immutable project audit';END IF;
 IF EXISTS(SELECT 1 FROM jsonb_array_elements(NEW.packages) p WHERE NOT EXISTS(SELECT 1 FROM purchase_sourcing_cases c WHERE c.package_id::text=p->>'id' AND c.project_id=NEW.project_id AND c.state='approved' AND c.data->>'award_reference'=NEW.attempt_id::text)) THEN RAISE EXCEPTION 'Award packages must carry exact approved source';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_finance_accounts_acceptance_event() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE p uuid;financial boolean;kind text;act text;BEGIN
 SELECT project_id INTO p FROM finance_accounts_acceptance WHERE id=NEW.subject_id;
 financial=NEW.after_snapshot->'acceptance'->>'status'='accepted' OR NEW.action='revise' AND NEW.before_snapshot->'acceptance'->>'status'='accepted';
 kind=CASE WHEN financial THEN 'financial_decision' ELSE 'accounts_acceptance' END;
 act=CASE WHEN NEW.after_snapshot->'acceptance'->>'status'='accepted' THEN 'approved' WHEN financial THEN 'returned' ELSE NEW.action END;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='finance_accounts_acceptance' AND entity_type=kind AND entity_id=NEW.subject_id AND entity_version=(NEW.after_snapshot->>'version')::integer AND action=act AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted' AND occurred_at=NEW.created_at) THEN RAISE EXCEPTION 'Accounts audit requires its exact central event';END IF;
 IF financial AND NOT EXISTS(SELECT 1 FROM communication_notices WHERE source_event_id=NEW.id AND decision_id=NEW.subject_id AND source_version=(NEW.after_snapshot->>'version')::integer AND decision_action=act AND project_id=p AND created_by_id=NEW.created_by_id) THEN RAISE EXCEPTION 'Accounts decision requires atomic Director notice';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_finance_accounts_acceptance_write() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM finance_accounts_acceptance_events WHERE subject_id=NEW.id AND after_snapshot=NEW.data AND created_by_id IS NOT DISTINCT FROM NEW.updated_by_id AND created_at=NEW.updated_at AND action=CASE WHEN TG_OP='UPDATE' AND NEW.revision>OLD.revision THEN 'revise' ELSE 'record' END AND (TG_OP='INSERT' AND before_snapshot='null'::jsonb OR TG_OP='UPDATE' AND before_snapshot=OLD.data)) THEN RAISE EXCEPTION 'Accounts acceptance mutation requires exact immutable audit';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_finance_advance_event() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='finance_approvals' AND entity_type='financial_decision' AND entity_id=NEW.id AND entity_version=1 AND action=NEW.outcome AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=NEW.project_id AND visibility='restricted' AND occurred_at=NEW.created_at) OR NOT EXISTS(SELECT 1 FROM communication_notices WHERE source_event_id=NEW.id AND decision_id=NEW.id AND decision_action=NEW.outcome AND project_id=NEW.project_id AND created_by_id=NEW.created_by_id) THEN RAISE EXCEPTION 'Advance Finance decision requires its immutable project event and Director notice';END IF;
 IF NOT EXISTS(SELECT 1 FROM purchase_advance_events WHERE subject_id=NEW.request_id AND after_snapshot->>'decision_id'=NEW.id::text AND (after_snapshot->>'submitted_version')::integer=NEW.source_version AND created_by_id=NEW.created_by_id AND action=CASE NEW.outcome WHEN 'approved' THEN 'approve' ELSE 'reject' END) THEN RAISE EXCEPTION 'Finance decision requires the consuming advance decision audit';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_finance_budget_event() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='finance_approvals' AND entity_type='financial_decision' AND entity_id=NEW.id AND entity_version=1 AND action='approved' AND actor_id=NEW.created_by_id AND scope_kind='project' AND scope_id=NEW.project_id AND visibility='restricted') THEN RAISE EXCEPTION 'Finance decision requires its immutable project event';END IF;
 RETURN NEW;
END $$;

CREATE FUNCTION public.check_finance_quotation_event() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='finance_approvals' AND entity_type='financial_decision' AND entity_id=NEW.id AND entity_version=1 AND action=NEW.outcome AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=NEW.project_id AND visibility='restricted') THEN RAISE EXCEPTION 'Quotation Finance decision requires its immutable project event';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_po_client_file_event() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE p uuid;
BEGIN
 SELECT o.project_id INTO p FROM purchase_orders o JOIN purchase_order_client_files f ON f.order_id=o.id WHERE f.id=NEW.file_id AND o.id=NEW.order_id;
 IF p IS NULL OR NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_orders' AND entity_type='purchase_order_client_file' AND entity_id=NEW.file_id AND action=NEW.action AND actor_id=NEW.created_by_id AND scope_id=p AND visibility='restricted') THEN RAISE EXCEPTION 'Signed PO evidence requires scoped immutable audit';END IF;
 RETURN NEW;
END $$;

CREATE FUNCTION public.check_po_client_file_write() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_orders o WHERE o.id=NEW.order_id AND o.state='draft' AND o.data->'client_approval'->>'status'='submitted' AND (o.data->>'draft_revision')::integer=NEW.draft_revision) THEN RAISE EXCEPTION 'Signed file requires matching submitted PO revision';END IF;
 IF NOT EXISTS(SELECT 1 FROM purchase_order_client_file_events WHERE file_id=NEW.id AND order_id=NEW.order_id AND action='uploaded' AND created_by_id=NEW.created_by_id AND created_at=NEW.created_at) THEN RAISE EXCEPTION 'Signed file upload requires audit';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_po_client_signoff() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE a jsonb;
BEGIN
 a=NEW.data->'client_approval';
 IF NEW.state='released' AND OLD.state<>'released' THEN
  IF coalesce(a->>'status','')<>'approved' OR coalesce((NEW.data->>'draft_revision')::integer,0)<1 OR a->>'draft_revision' IS DISTINCT FROM NEW.data->>'draft_revision' OR a->'decision'->>'status' IS DISTINCT FROM 'approved' OR coalesce(a->>'recorded_by_id','')='' OR a->>'recorded_at' IS NULL THEN RAISE EXCEPTION 'Client sign-off for this draft is required before PO release'; END IF;
  IF NOT EXISTS(SELECT 1 FROM purchase_order_client_files f WHERE f.id::text=a->'decision'->>'file_id' AND f.order_id=NEW.id AND f.draft_revision=(NEW.data->>'draft_revision')::integer AND f.is_active) THEN RAISE EXCEPTION 'Actual signed PO file must match this order revision';END IF;
  IF NEW.data->'client_approval' IS DISTINCT FROM OLD.data->'client_approval' THEN RAISE EXCEPTION 'Release must retain previously recorded client approval';END IF;
 END IF;
 IF NEW.data->'details' IS DISTINCT FROM OLD.data->'details' OR NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot THEN
  IF coalesce(a->>'status','')='approved' THEN RAISE EXCEPTION 'Edited PO must obtain new client approval';END IF;
 END IF;
 RETURN NEW;
END $$;

CREATE FUNCTION public.check_project_budget_finance_baseline() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE d finance_budget_decisions;BEGIN
 IF NEW.approval_decision_id IS NOT NULL THEN
  SELECT * INTO d FROM finance_budget_decisions WHERE id=NEW.approval_decision_id;
  IF NOT FOUND OR d.project_id IS DISTINCT FROM NEW.project_id OR d.source_revision IS DISTINCT FROM (NEW.approved_snapshot->>'revision')::integer OR d.source_revision>NEW.revision OR d.currency IS DISTINCT FROM (NEW.approved_snapshot->'data'->>'currency') OR d.amount IS DISTINCT FROM (NEW.approved_snapshot->'data'->>'total')::numeric OR d.created_by_id::text IS DISTINCT FROM (NEW.approved_snapshot->>'approved_by_id') OR d.id::text IS DISTINCT FROM (NEW.approved_snapshot->>'decision_id') THEN RAISE EXCEPTION 'Approved baseline requires its matching Finance decision';END IF;
 END IF;
 IF TG_OP='UPDATE' THEN
  IF NEW.approval_decision_id IS NOT DISTINCT FROM OLD.approval_decision_id AND NEW.approved_snapshot IS DISTINCT FROM OLD.approved_snapshot THEN RAISE EXCEPTION 'Approved budget baseline is immutable';END IF;
  IF OLD.state='approved' AND NEW.state='approved' THEN RAISE EXCEPTION 'Start a budget revision before changing an approved baseline';END IF;
  IF OLD.state='approved' AND (NEW.state<>'draft' OR NEW.revision<>OLD.revision+1 OR NEW.approval_decision_id IS DISTINCT FROM OLD.approval_decision_id OR NEW.approved_snapshot IS DISTINCT FROM OLD.approved_snapshot) THEN RAISE EXCEPTION 'A revision must retain the prior approved baseline';END IF;
 END IF;
 IF NEW.state='approved' AND (d.source_version<>NEW.version-1 OR d.source_revision<>NEW.revision OR d.amount<>NEW.total OR d.currency<>NEW.currency OR NEW.updated_by_id IS DISTINCT FROM d.created_by_id) THEN RAISE EXCEPTION 'Budget approval requires its current source version and actor';END IF;
 RETURN NEW;
END $$;

CREATE FUNCTION public.check_purchase_acceptance_event() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE p uuid;BEGIN
 SELECT project_id INTO p FROM purchase_acceptances WHERE id=NEW.subject_id;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_acceptance' AND entity_type='purchase_acceptance' AND entity_id=NEW.subject_id AND entity_version=(NEW.after_snapshot->>'version')::integer AND action=NEW.action AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted') THEN RAISE EXCEPTION 'Acceptance audit requires central project event';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_acceptance_write() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_acceptance_events WHERE subject_id=NEW.id AND after_snapshot=NEW.data AND created_by_id IS NOT DISTINCT FROM NEW.updated_by_id AND created_at=NEW.updated_at AND (TG_OP='INSERT' AND before_snapshot='null'::jsonb OR TG_OP='UPDATE' AND before_snapshot=OLD.data)) THEN RAISE EXCEPTION 'Acceptance mutation requires exact immutable audit';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_advance_event() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE p uuid;BEGIN
 SELECT project_id INTO p FROM purchase_advances WHERE id=NEW.subject_id;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_advances' AND entity_type='purchase_advance' AND entity_id=NEW.subject_id AND entity_version=(NEW.after_snapshot->>'version')::integer AND action=NEW.action AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted' AND occurred_at=NEW.created_at) THEN RAISE EXCEPTION 'Advance audit requires central project event';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_advance_write() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE e purchase_advance_events;f finance_advance_decisions;BEGIN
 SELECT * INTO e FROM purchase_advance_events WHERE subject_id=NEW.id AND after_snapshot=NEW.data AND created_by_id=NEW.updated_by_id AND created_at=NEW.updated_at;
 IF e.id IS NULL OR (TG_OP='INSERT' AND (e.before_snapshot<>'null'::jsonb OR e.action<>'prepare')) OR (TG_OP='UPDATE' AND e.before_snapshot IS DISTINCT FROM OLD.data) THEN RAISE EXCEPTION 'Advance mutation requires exact immutable audit';END IF;
 IF (e.action='prepare' AND NEW.state<>'draft') OR (e.action='submit' AND NEW.state<>'requested') OR (e.action='revise' AND NEW.state<>'draft') OR (e.action='approve' AND NEW.state NOT IN('approved','not_applicable')) OR (e.action='reject' AND NEW.state<>'rejected') OR (e.action='disburse' AND NEW.state<>'disbursed') THEN RAISE EXCEPTION 'Advance audit action differs from resulting state';END IF;
 IF NEW.state IN('approved','rejected','not_applicable','disbursed') THEN
  SELECT * INTO f FROM finance_advance_decisions WHERE id=NEW.decision_id;
  IF f.id IS NULL OR ROW(f.request_id,f.project_id,f.package_id,f.order_id,f.order_version,f.source_version,f.source_fingerprint) IS DISTINCT FROM ROW(NEW.id,NEW.project_id,NEW.package_id,NEW.order_id,NEW.order_version,(NEW.data->>'submitted_version')::integer,NEW.source_fingerprint::char(64)) OR f.amount IS DISTINCT FROM (NEW.data->>'amount')::numeric OR f.currency IS DISTINCT FROM NEW.source_snapshot->>'currency' OR f.kind IS DISTINCT FROM NEW.data->'details'->>'kind' OR f.submitted_by_id::text IS DISTINCT FROM NEW.data->>'submitted_by_id' OR f.created_by_id::text IS DISTINCT FROM NEW.data->>'reviewed_by_id' OR NEW.data->'reviewed_at'='null'::jsonb OR (NEW.state='rejected') IS DISTINCT FROM (f.outcome='rejected') OR (NEW.state='not_applicable') IS DISTINCT FROM (f.kind='not_applicable' AND f.outcome='approved') THEN RAISE EXCEPTION 'Advance outcome requires its matching Finance decision';END IF;
 END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_client_clearance_event() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE p uuid;BEGIN
 SELECT project_id INTO p FROM purchase_client_clearances WHERE id=NEW.subject_id;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_client_clearance' AND entity_type='purchase_client_clearance' AND entity_id=NEW.subject_id AND entity_version=(NEW.after_snapshot->>'version')::integer AND action=NEW.action AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted' AND occurred_at=NEW.created_at) THEN RAISE EXCEPTION 'Report audit requires central project event';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_client_clearance_write() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_client_clearance_events WHERE subject_id=NEW.id AND after_snapshot=NEW.data AND created_by_id IS NOT DISTINCT FROM NEW.updated_by_id AND created_at=NEW.updated_at AND action=CASE WHEN TG_OP='UPDATE' AND NEW.revision>OLD.revision THEN 'revise' ELSE 'record' END AND (TG_OP='INSERT' AND before_snapshot='null'::jsonb OR TG_OP='UPDATE' AND before_snapshot=OLD.data)) THEN RAISE EXCEPTION 'Report mutation requires exact immutable audit';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_dispatch_event() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE p uuid;BEGIN
 SELECT project_id INTO p FROM purchase_dispatches WHERE id=NEW.subject_id;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_dispatch' AND entity_type='purchase_dispatch' AND entity_id=NEW.subject_id AND entity_version=(NEW.after_snapshot->>'version')::integer AND action=NEW.action AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted' AND occurred_at=NEW.created_at) THEN RAISE EXCEPTION 'Report audit requires central project event';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_dispatch_write() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_dispatch_events WHERE subject_id=NEW.id AND after_snapshot=NEW.data AND created_by_id IS NOT DISTINCT FROM NEW.updated_by_id AND created_at=NEW.updated_at AND action=CASE WHEN TG_OP='UPDATE' AND NEW.revision>OLD.revision THEN 'revise' ELSE 'record' END AND (TG_OP='INSERT' AND before_snapshot='null'::jsonb OR TG_OP='UPDATE' AND before_snapshot=OLD.data)) THEN RAISE EXCEPTION 'Report mutation requires exact immutable audit';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_engineer_confirmations_event() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE p uuid;BEGIN
 SELECT project_id INTO p FROM purchase_engineer_confirmations WHERE id=NEW.subject_id;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_engineer_confirmations' AND entity_type='purchase_engineer_confirmations' AND entity_id=NEW.subject_id AND entity_version=(NEW.after_snapshot->>'version')::integer AND action=NEW.action AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted' AND occurred_at=NEW.created_at) THEN RAISE EXCEPTION 'Engineer confirmation audit requires central project event';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_engineer_confirmations_write() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_engineer_confirmations_events WHERE subject_id=NEW.id AND after_snapshot=NEW.data AND created_by_id IS NOT DISTINCT FROM NEW.updated_by_id AND created_at=NEW.updated_at AND action=CASE WHEN TG_OP='UPDATE' AND NEW.revision>OLD.revision THEN 'revise' ELSE 'record' END AND (TG_OP='INSERT' AND before_snapshot='null'::jsonb OR TG_OP='UPDATE' AND before_snapshot=OLD.data)) THEN RAISE EXCEPTION 'Engineer confirmation mutation requires exact immutable audit';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_grns_event() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE p uuid;BEGIN
 SELECT project_id INTO p FROM purchase_grns WHERE id=NEW.subject_id;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_grns' AND entity_type='purchase_grns' AND entity_id=NEW.subject_id AND entity_version=(NEW.after_snapshot->>'version')::integer AND action=NEW.action AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted' AND occurred_at=NEW.created_at) THEN RAISE EXCEPTION 'GRN audit requires central project event';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_grns_write() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_grns_events WHERE subject_id=NEW.id AND after_snapshot=NEW.data AND created_by_id IS NOT DISTINCT FROM NEW.updated_by_id AND created_at=NEW.updated_at AND action=CASE WHEN TG_OP='UPDATE' AND NEW.revision>OLD.revision THEN 'revise' ELSE 'record' END AND (TG_OP='INSERT' AND before_snapshot='null'::jsonb OR TG_OP='UPDATE' AND before_snapshot=OLD.data)) THEN RAISE EXCEPTION 'GRN mutation requires exact immutable audit';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_inspection_event() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE p uuid;BEGIN
 SELECT project_id INTO p FROM purchase_inspections WHERE id=NEW.subject_id;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_inspections' AND entity_type='purchase_inspections' AND entity_id=NEW.subject_id AND entity_version=(NEW.after_snapshot->>'version')::integer AND action=NEW.action AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted' AND occurred_at=NEW.created_at) THEN RAISE EXCEPTION 'Inspection audit requires central project event';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_inspection_report_event() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE p uuid;BEGIN
 SELECT project_id INTO p FROM purchase_inspection_reports WHERE id=NEW.subject_id;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_inspection_reports' AND entity_type='purchase_inspection_reports' AND entity_id=NEW.subject_id AND entity_version=(NEW.after_snapshot->>'version')::integer AND action=NEW.action AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted' AND occurred_at=NEW.created_at) THEN RAISE EXCEPTION 'Report audit requires central project event';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_inspection_report_write() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_inspection_report_events WHERE subject_id=NEW.id AND after_snapshot=NEW.data AND created_by_id IS NOT DISTINCT FROM NEW.updated_by_id AND created_at=NEW.updated_at AND action=CASE WHEN TG_OP='UPDATE' AND NEW.revision>OLD.revision THEN 'revise' ELSE 'record' END AND (TG_OP='INSERT' AND before_snapshot='null'::jsonb OR TG_OP='UPDATE' AND before_snapshot=OLD.data)) THEN RAISE EXCEPTION 'Report mutation requires exact immutable audit';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_inspection_write() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_inspection_events WHERE subject_id=NEW.id AND after_snapshot=NEW.data AND created_by_id IS NOT DISTINCT FROM NEW.updated_by_id AND created_at=NEW.updated_at AND action=CASE WHEN TG_OP='UPDATE' AND NEW.revision>OLD.revision THEN 'revise' ELSE 'record' END AND (TG_OP='INSERT' AND before_snapshot='null'::jsonb OR TG_OP='UPDATE' AND before_snapshot=OLD.data)) THEN RAISE EXCEPTION 'Inspection mutation requires exact immutable audit';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_loi_audit() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM business_events e WHERE e.id=NEW.id AND e.source_module='purchase_awards' AND e.entity_type='vendor_loi_document_event' AND e.entity_id=NEW.id AND e.entity_version=1 AND e.action=NEW.action AND e.actor_id=NEW.created_by_id AND e.actor_kind='user' AND e.scope_kind='project' AND e.visibility='restricted' AND e.scope_id=(SELECT project_id FROM purchase_loi_documents WHERE id=NEW.document_id)) THEN RAISE EXCEPTION 'LOI access requires immutable project audit';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_loi_generation() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_loi_document_events e WHERE e.document_id=NEW.id AND e.action='generated' AND e.created_by_id=NEW.created_by_id AND e.created_at=NEW.created_at) THEN RAISE EXCEPTION 'LOI generation requires its audit';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_loi_handoff() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_award_handoffs h WHERE h.id=NEW.handoff_id AND h.project_id=NEW.project_id AND h.attempt_id=NEW.attempt_id AND h.is_active) THEN RAISE EXCEPTION 'LOI must belong to its approved award handoff';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_loi_review_audit() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM business_events e WHERE e.id=NEW.id AND e.source_module='purchase_awards' AND e.entity_type='loi_document_review' AND e.entity_id=NEW.id AND e.entity_version=1 AND e.action='completed' AND e.actor_id=NEW.created_by_id AND e.actor_kind='user' AND e.scope_kind='project' AND e.visibility='restricted' AND e.scope_id=(SELECT project_id FROM purchase_loi_documents WHERE id=NEW.document_id)) THEN RAISE EXCEPTION 'LOI review requires immutable project audit'; END IF;
 RETURN NEW; END $$;

CREATE FUNCTION public.check_purchase_order_document() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE p uuid;BEGIN
 SELECT project_id INTO p FROM purchase_orders WHERE id=NEW.order_id AND version=NEW.order_version;
 IF p IS NULL OR NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_orders' AND entity_type='purchase_order_document_event' AND entity_id=NEW.id AND entity_version=1 AND action='print_requested' AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted') THEN RAISE EXCEPTION 'Order document export requires current version and immutable project attribution';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_order_event() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE p uuid;BEGIN
 SELECT project_id INTO p FROM purchase_orders WHERE id=NEW.subject_id;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_orders' AND entity_type='purchase_order' AND entity_id=NEW.subject_id AND entity_version=(NEW.after_snapshot->>'version')::integer AND action=NEW.action AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted') THEN RAISE EXCEPTION 'Order audit requires immutable central project event';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_order_write() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_order_events WHERE subject_id=NEW.id AND after_snapshot=NEW.data AND created_by_id IS NOT DISTINCT FROM NEW.updated_by_id AND created_at=NEW.updated_at AND (TG_OP='INSERT' OR before_snapshot=OLD.data)) THEN RAISE EXCEPTION 'Order mutation requires exact immutable audit';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_quality_children() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE k uuid;c purchase_quality_cases;BEGIN
 IF TG_TABLE_NAME='purchase_quality_cases' THEN k=NEW.id;ELSE k=NEW.case_id;END IF;
 SELECT * INTO c FROM purchase_quality_cases WHERE id=k;
 IF (SELECT count(*) FROM purchase_quality_certificates WHERE case_id=k)<>5 OR EXISTS(SELECT 1 FROM purchase_quality_certificates child WHERE child.case_id=k AND NOT EXISTS(SELECT 1 FROM jsonb_array_elements(c.data->'certificates') x WHERE x=child.data)) THEN RAISE EXCEPTION 'Quality child records must equal the audited case snapshot';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_quality_event() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE p uuid;BEGIN
 SELECT project_id INTO p FROM purchase_quality_cases WHERE id=NEW.subject_id;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_quality' AND entity_type='purchase_quality' AND entity_id=NEW.subject_id AND entity_version=(NEW.after_snapshot->>'version')::integer AND action=NEW.action AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted' AND occurred_at=NEW.created_at) THEN RAISE EXCEPTION 'Quality audit requires central project event';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_quality_write() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_quality_events WHERE subject_id=NEW.id AND after_snapshot=NEW.data AND created_by_id=NEW.updated_by_id AND created_at=NEW.updated_at AND (TG_OP='INSERT' AND action='record' AND before_snapshot='null'::jsonb OR TG_OP='UPDATE' AND before_snapshot=OLD.data AND (NEW.revision=OLD.revision AND action IN('record','verify','revoke') OR NEW.revision=OLD.revision+1 AND action='revise'))) THEN RAISE EXCEPTION 'Quality mutation requires exact immutable audit';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_site_receipts_event() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE p uuid;BEGIN
 SELECT project_id INTO p FROM purchase_site_receipts WHERE id=NEW.subject_id;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_site_receipts' AND entity_type='purchase_site_receipts' AND entity_id=NEW.subject_id AND entity_version=(NEW.after_snapshot->>'version')::integer AND action=NEW.action AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted' AND occurred_at=NEW.created_at) THEN RAISE EXCEPTION 'Site receipt audit requires central project event';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_site_receipts_write() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_site_receipts_events WHERE subject_id=NEW.id AND after_snapshot=NEW.data AND created_by_id IS NOT DISTINCT FROM NEW.updated_by_id AND created_at=NEW.updated_at AND action=CASE WHEN TG_OP='UPDATE' AND NEW.revision>OLD.revision THEN 'revise' ELSE 'record' END AND (TG_OP='INSERT' AND before_snapshot='null'::jsonb OR TG_OP='UPDATE' AND before_snapshot=OLD.data)) THEN RAISE EXCEPTION 'Site receipt mutation requires exact immutable audit';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_sourcing_allocation() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE c purchase_sourcing_cases;BEGIN
 SELECT * INTO c FROM purchase_sourcing_cases WHERE id=NEW.case_id;
 IF c.version<>NEW.case_version OR c.updated_at<>NEW.updated_at OR c.updated_by_id IS DISTINCT FROM NEW.updated_by_id OR (NEW.is_active AND (c.state<>NEW.state OR c.source_snapshot->>'currency'<>NEW.currency OR (c.source_snapshot->>'quantity')::numeric<>NEW.quantity OR c.data->'decision_snapshot'->>'financial_reference' IS DISTINCT FROM NEW.financial_reference)) OR (NOT NEW.is_active AND c.state<>'draft') THEN RAISE EXCEPTION 'Allocation requires matching current sourcing state and audit';END IF;
 IF NOT EXISTS(SELECT 1 FROM purchase_sourcing_events WHERE subject_id=NEW.case_id AND (after_snapshot->>'version')::integer=NEW.case_version AND created_by_id=NEW.updated_by_id AND created_at=NEW.updated_at) THEN RAISE EXCEPTION 'Allocation requires exact sourcing history';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_sourcing_event() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE c purchase_sourcing_cases;BEGIN
 SELECT * INTO c FROM purchase_sourcing_cases WHERE id=NEW.subject_id;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_sourcing' AND entity_type='sourcing_case' AND entity_id=NEW.subject_id AND entity_version=(NEW.after_snapshot->>'version')::integer AND action=NEW.action AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=c.project_id AND visibility='restricted') THEN RAISE EXCEPTION 'Sourcing history requires its immutable project event';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_sourcing_reservation() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NEW.state<>'draft' AND NOT EXISTS(SELECT 1 FROM purchase_sourcing_allocations WHERE case_id=NEW.id AND is_active AND state=NEW.state AND case_version=NEW.version) THEN RAISE EXCEPTION 'Submitted sourcing requires its owning allocation';END IF;
 IF NEW.state='draft' AND EXISTS(SELECT 1 FROM purchase_sourcing_allocations WHERE case_id=NEW.id AND is_active) AND NOT EXISTS(SELECT 1 FROM purchase_sourcing_cases c WHERE c.id=NEW.id AND c.version>NEW.version AND c.state IN('approved','released') AND coalesce(c.data->>'award_reference','')<>'') THEN RAISE EXCEPTION 'Draft sourcing cannot retain a reservation';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_sourcing_write() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_sourcing_events WHERE subject_id=NEW.id AND after_snapshot=NEW.data AND created_by_id=NEW.updated_by_id AND created_at=NEW.updated_at AND (TG_OP='INSERT' OR before_snapshot=OLD.data)) THEN RAISE EXCEPTION 'Every sourcing write requires its exact immutable history';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_technical_event() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE p uuid;BEGIN
 SELECT project_id INTO p FROM purchase_technical_cases WHERE id=NEW.subject_id;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_technical' AND entity_type='technical_case' AND entity_id=NEW.subject_id AND entity_version=(NEW.after_snapshot->>'version')::integer AND action=NEW.action AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted') THEN RAISE EXCEPTION 'Technical audit requires immutable central project event';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_technical_file() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_technical_file_events WHERE file_id=NEW.id AND case_id=NEW.case_id AND action='uploaded' AND created_at=NEW.created_at AND created_by_id IS NOT DISTINCT FROM NEW.created_by_id) THEN RAISE EXCEPTION 'Technical upload requires immutable file audit';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_technical_file_event() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE p uuid;BEGIN
 SELECT project_id INTO p FROM purchase_technical_cases WHERE id=NEW.case_id;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_technical' AND entity_type='technical_file_event' AND entity_id=NEW.id AND entity_version=1 AND action=NEW.action AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted') THEN RAISE EXCEPTION 'Technical file audit requires central project event';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_purchase_technical_write() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_technical_events WHERE subject_id=NEW.id AND after_snapshot=NEW.data AND created_by_id IS NOT DISTINCT FROM NEW.updated_by_id AND created_at=NEW.updated_at AND (TG_OP='INSERT' OR before_snapshot=OLD.data)) THEN RAISE EXCEPTION 'Technical write requires exact immutable audit';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_quotation_approval_decisions() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE a jsonb;d finance_quotation_decisions;BEGIN
 FOR a IN SELECT value FROM jsonb_array_elements(NEW.attempts) LOOP
  IF a->>'state' IN('approved','rejected') THEN
   IF EXISTS(SELECT 1 FROM jsonb_array_elements(a->'lines') l WHERE l->>'decision' NOT IN('accept','reject') OR l->>'decision' IS NULL OR coalesce(length(l->>'comment'),0)>2000) OR (a->>'state'='rejected') IS DISTINCT FROM EXISTS(SELECT 1 FROM jsonb_array_elements(a->'lines') l WHERE l->>'decision'='reject') THEN RAISE EXCEPTION 'Every item decision must match the Finance outcome';END IF;
   SELECT * INTO d FROM finance_quotation_decisions WHERE id=(a->>'decision_id')::uuid AND attempt_id=(a->>'id')::uuid AND bo_id=NEW.bo_id AND project_id=NEW.project_id;
   IF NOT FOUND OR d.outcome IS DISTINCT FROM a->>'state' OR d.currency IS DISTINCT FROM a->>'currency' OR d.amount IS DISTINCT FROM (a->>'total')::numeric OR d.submitted_by_id::text IS DISTINCT FROM a->>'submitted_by_id' OR d.created_by_id::text IS DISTINCT FROM a->>'reviewed_by_id' OR d.source_version>=NEW.version OR a->>'reviewed_at' IS NULL THEN RAISE EXCEPTION 'Quotation outcome requires its matching Finance decision';END IF;
  ELSIF a->>'state' IS DISTINCT FROM 'pending_accounts' OR a->>'decision_id' IS NOT NULL OR a->>'reviewed_by_id' IS NOT NULL OR a->>'reviewed_at' IS NOT NULL THEN RAISE EXCEPTION 'Quotation request state is invalid';END IF;
 END LOOP;
 RETURN NEW;END $$;

CREATE FUNCTION public.check_quotation_shortlist_sources() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE a jsonb;p jsonb;r jsonb;BEGIN
 FOR a IN SELECT value FROM jsonb_array_elements(NEW.attempts) LOOP
  IF jsonb_array_length(coalesce(a->'proposals','[]'))>0 THEN
   IF jsonb_array_length(a->'proposals')>2 OR (SELECT count(*) FROM jsonb_array_elements(a->'proposals') x WHERE x->>'best_price'='true')<>1 OR (SELECT count(*) FROM jsonb_array_elements(a->'proposals') x WHERE x->>'preferred'='true')<>1 THEN RAISE EXCEPTION 'Exactly one best-price and preferred nomination required';END IF;
   FOR p IN SELECT value FROM jsonb_array_elements(a->'proposals') LOOP
    r:=p->'snapshot';
    IF (p->>'best_price'='true' AND coalesce(length(btrim(p->>'best_price_comment')),0)=0) OR (p->>'preferred'='true' AND coalesce(length(btrim(p->>'preferred_comment')),0)=0) THEN RAISE EXCEPTION 'Designation reasons required';END IF;
    IF r->>'group_id' IS DISTINCT FROM a->>'group_id' OR r->>'scope_revision' IS DISTINCT FROM a->>'scope_revision' OR p->>'quote_key' IS DISTINCT FROM r->>'quote_key' OR NOT EXISTS(SELECT 1 FROM purchase_quotation_revisions q JOIN purchase_indent_documents d ON d.id=q.document_id WHERE q.bo_id=NEW.bo_id AND d.project_id=NEW.project_id AND d.group_id::text=a->>'group_id' AND q.document_id::text=r->>'document_id' AND q.revision=(r->>'revision')::integer AND d.vendor_id::text=r->>'vendor_id' AND q.invoice_total=(r->>'total')::numeric) THEN RAISE EXCEPTION 'Submitted proposal requires its owned immutable quotation revision';END IF;
   END LOOP;
  END IF;
 END LOOP;
 RETURN NEW;END $$;

CREATE FUNCTION public.epcerp_preserve_completed_document_task() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 IF TG_OP='DELETE' OR OLD.completed_at IS NOT NULL THEN RAISE EXCEPTION 'Completed document task evidence is immutable';END IF;
 RETURN NEW;
END $$;

CREATE FUNCTION public.epcerp_reject_event_mutation() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 RAISE EXCEPTION 'Immutable event evidence cannot be changed' USING ERRCODE='55000';
END;
$$;

CREATE FUNCTION public.protect_catalogue_item_evidence() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Item history and issued items cannot be deleted or rewritten'; END $$;

CREATE FUNCTION public.protect_catalogue_item_identity() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF ROW(NEW.configuration_id,NEW.manufacturer_id,NEW.category_id,NEW.model,NEW.specifications,NEW.snapshot,NEW.fingerprint,NEW.engineering_key,NEW.description,NEW.sku,NEW.serial,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.configuration_id,OLD.manufacturer_id,OLD.category_id,OLD.model,OLD.specifications,OLD.snapshot,OLD.fingerprint,OLD.engineering_key,OLD.description,OLD.sku,OLD.serial,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Issued item identity is immutable'; END IF; RETURN NEW; END $$;

CREATE FUNCTION public.protect_client_purchase_rule_history() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Client purchase policy evidence is immutable; deactivate instead'; END $$;

CREATE FUNCTION public.protect_communication_notice() RETURNS trigger
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

CREATE FUNCTION public.protect_consumed_package() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF TG_OP='TRUNCATE' THEN RAISE EXCEPTION 'Package evidence requires retention-aware cleanup'; END IF;
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Package evidence cannot be deleted'; END IF;
 IF OLD.state='consumed' THEN RAISE EXCEPTION 'Consumed package is immutable'; END IF;
 IF ROW(NEW.id,NEW.owner_id,NEW.purpose,NEW.limit_profile_revision,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.id,OLD.owner_id,OLD.purpose,OLD.limit_profile_revision,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Package identity is immutable'; END IF;
 IF OLD.draft_id IS NOT NULL AND NEW.draft_id IS DISTINCT FROM OLD.draft_id THEN RAISE EXCEPTION 'Retained draft is immutable'; END IF;
 IF NEW.expires_at IS DISTINCT FROM OLD.expires_at AND NOT(OLD.draft_id IS NULL AND NEW.draft_id IS NOT NULL AND NEW.expires_at='9999-12-31'::timestamptz) THEN RAISE EXCEPTION 'Package expiry is immutable'; END IF;
 RETURN NEW; END $$;

CREATE FUNCTION public.protect_finance_accounts_acceptance() RETURNS trigger
    LANGUAGE plpgsql
    AS $_$ DECLARE w purchase_engineer_confirmations;o purchase_orders;z purchase_grns;y purchase_site_receipts;x purchase_dispatches;c purchase_client_clearances;q purchase_quality_cases;r purchase_inspection_reports;i purchase_inspections;d jsonb;expected jsonb;BEGIN
 SELECT * INTO w FROM purchase_engineer_confirmations WHERE id=NEW.engineer_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=NEW.engineer_version AND status='confirmed' AND is_active;
 IF w.id IS NULL THEN RAISE EXCEPTION 'Accounts acceptance needs current confirmed engineer evidence';END IF;
 SELECT * INTO z FROM purchase_grns WHERE id=w.grn_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=w.grn_version AND status='issued' AND is_active;
 IF z.id IS NULL THEN RAISE EXCEPTION 'Accounts acceptance needs current issued GRN';END IF;
 SELECT * INTO o FROM purchase_orders WHERE id=(w.source_snapshot->>'order_id')::uuid AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=(w.source_snapshot->>'order_version')::integer AND state='released' AND is_active;
 IF o.id IS NULL THEN RAISE EXCEPTION 'Accounts acceptance needs its actual released PO';END IF;
 SELECT * INTO y FROM purchase_site_receipts WHERE id=z.receipt_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=z.receipt_version AND status='delivered_at_site' AND is_active;
 IF y.id IS NULL THEN RAISE EXCEPTION 'Accounts acceptance needs the current delivered site receipt';END IF;
 SELECT * INTO x FROM purchase_dispatches WHERE id=y.dispatch_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=y.dispatch_version AND status='dispatched' AND is_active;
 IF x.id IS NULL THEN RAISE EXCEPTION 'GRN needs its current dispatched material';END IF;
 SELECT * INTO c FROM purchase_client_clearances WHERE id=x.clearance_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=x.clearance_version AND status='cleared' AND is_active;
 IF c.id IS NULL THEN RAISE EXCEPTION 'GRN needs its current cleared client MDCC';END IF;
 SELECT * INTO q FROM purchase_quality_cases WHERE id=c.quality_id AND version=c.quality_version AND is_active;
 IF q.id IS NULL OR (SELECT count(*) FROM purchase_quality_certificates WHERE case_id=q.id)<>5 OR EXISTS(SELECT 1 FROM purchase_quality_certificates WHERE case_id=q.id AND (NOT is_active OR NOT verified OR valid_until IS NOT NULL AND valid_until<(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date)) THEN RAISE EXCEPTION 'GRN requires current verified quality';END IF;
 SELECT * INTO r FROM purchase_inspection_reports WHERE id=q.report_id AND version=q.report_version AND status='issued' AND is_active;
 IF r.id IS NULL THEN RAISE EXCEPTION 'GRN needs current issued IRN';END IF;
 SELECT * INTO i FROM purchase_inspections WHERE id=r.inspection_id AND version=r.inspection_version AND status='cleared' AND is_active;
 IF i.id IS NULL OR NOT EXISTS(SELECT 1 FROM purchase_acceptances WHERE id=i.acceptance_id AND version=i.acceptance_version AND status='accepted' AND is_active) OR NOT EXISTS(SELECT 1 FROM purchase_technical_cases WHERE id=i.technical_id AND version=i.technical_version AND state IN('approved','approved_with_comments') AND is_active) THEN RAISE EXCEPTION 'GRN source evidence is no longer current';END IF;
 expected=w.source_snapshot||jsonb_build_object('engineer_id',w.id,'engineer_version',w.version,'engineer_revision',w.revision,'confirmation_date',w.data->'confirmation'->>'confirmation_date','engineer_name',w.data->'confirmation'->>'engineer_name','designation',w.data->'confirmation'->>'designation','mbook_ref',w.data->'confirmation'->>'mbook_ref','work_package_ref',w.data->'confirmation'->>'work_package_ref','engineer_remarks',w.data->'confirmation'->>'remarks','currency',o.source_snapshot->>'currency');
 IF NEW.source_snapshot IS DISTINCT FROM expected THEN RAISE EXCEPTION 'Accounts acceptance source must equal current engineer and released PO';END IF;
 IF NEW.data->>'id' IS DISTINCT FROM NEW.id::text OR NEW.data->>'project_id' IS DISTINCT FROM NEW.project_id::text OR NEW.data->>'package_id' IS DISTINCT FROM NEW.package_id::text OR NEW.data->>'version' IS DISTINCT FROM NEW.version::text OR NEW.data->>'revision' IS DISTINCT FROM NEW.revision::text OR NEW.data->>'is_active' IS DISTINCT FROM NEW.is_active::text OR NEW.data->'source_snapshot' IS DISTINCT FROM NEW.source_snapshot OR NEW.data->'acceptance'->>'status' IS DISTINCT FROM NEW.status OR (NEW.data->>'created_at')::timestamptz IS DISTINCT FROM NEW.created_at OR (NEW.data->>'updated_at')::timestamptz IS DISTINCT FROM NEW.updated_at OR nullif(NEW.data->>'created_by_id','')::uuid IS DISTINCT FROM NEW.created_by_id OR nullif(NEW.data->>'updated_by_id','')::uuid IS DISTINCT FROM NEW.updated_by_id THEN RAISE EXCEPTION 'Accounts acceptance projection mismatch';END IF;
 IF NEW.created_by_id IS NULL OR NEW.updated_by_id IS NULL THEN RAISE EXCEPTION 'Use authenticated Finance attribution';END IF;
 d=NEW.data->'acceptance';
 IF EXISTS(SELECT 1 FROM unnest(ARRAY['status','acceptance_date','officer_name','voucher_ref','booked_amount','notes']) k WHERE jsonb_typeof(d->k) IS DISTINCT FROM 'string') OR (SELECT count(*) FROM jsonb_object_keys(d))<>6 THEN RAISE EXCEPTION 'Use declared Accounts fields as strings';END IF;
 IF NEW.status='accepted' AND (length(trim(d->>'officer_name'))=0 OR d->>'acceptance_date'='') THEN RAISE EXCEPTION 'Accepted needs actual officer and date';END IF;
 IF length(d->>'officer_name')>300 OR length(d->>'voucher_ref')>200 OR length(d->>'notes')>4000 OR d->>'booked_amount' !~ '^(0|[1-9][0-9]{0,11})(\.[0-9]{1,2})?$' THEN RAISE EXCEPTION 'Accounts fields exceed limits';END IF;
 IF d->>'acceptance_date'<>'' AND ((d->>'acceptance_date')::date<(w.data->'confirmation'->>'confirmation_date')::date OR (d->>'acceptance_date')::date>(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date) THEN RAISE EXCEPTION 'Acceptance follows engineer confirmation and cannot be future';END IF;
 IF TG_OP='INSERT' AND (NEW.version<>1 OR NEW.revision<>1) THEN RAISE EXCEPTION 'Accounts acceptance begins at version/revision one';END IF;
 IF TG_OP='UPDATE' THEN
 IF NEW.id<>OLD.id OR NEW.project_id<>OLD.project_id OR NEW.package_id<>OLD.package_id OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'Accounts acceptance ownership/audit must be retained';END IF;
 IF NEW.revision=OLD.revision THEN
 IF OLD.status='accepted' OR NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot THEN RAISE EXCEPTION 'Accepted or obsolete Accounts evidence needs revision';END IF;
 ELSIF NEW.revision<>OLD.revision+1 OR NEW.status<>'pending' OR d IS DISTINCT FROM jsonb_build_object('status','pending','acceptance_date','','officer_name','','voucher_ref','','booked_amount','0','notes','') THEN RAISE EXCEPTION 'Accounts acceptance revision resets pending and retains prior evidence';END IF;
 END IF;
 RETURN NEW;END $_$;

CREATE FUNCTION public.protect_finance_accounts_acceptance_history() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Accounts acceptance and history cannot be erased';END $$;

CREATE FUNCTION public.protect_finance_advance_source() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE r purchase_advances;BEGIN
 SELECT * INTO r FROM purchase_advances WHERE id=NEW.request_id FOR UPDATE;
 IF r.id IS NULL OR r.state<>'requested' OR ROW(r.project_id,r.package_id,r.order_id,r.order_version) IS DISTINCT FROM ROW(NEW.project_id,NEW.package_id,NEW.order_id,NEW.order_version) OR r.version<>NEW.source_version OR r.source_fingerprint IS DISTINCT FROM NEW.source_fingerprint OR (r.data->>'amount')::numeric IS DISTINCT FROM NEW.amount OR r.source_snapshot->>'currency' IS DISTINCT FROM NEW.currency OR r.data->>'submitted_by_id' IS DISTINCT FROM NEW.submitted_by_id::text OR r.data->'details'->>'kind' IS DISTINCT FROM NEW.kind THEN RAISE EXCEPTION 'Finance reviews the exact current submitted advance';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.protect_finance_budget_decisions() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Finance decisions are immutable';END $$;

CREATE FUNCTION public.protect_indent_document_evidence() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Vendor indent evidence cannot be deleted or rewritten'; END $$;

CREATE FUNCTION public.protect_indent_document_scope() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
IF ROW(NEW.id,NEW.project_id,NEW.bo_id,NEW.group_id,NEW.vendor_id,NEW.scope_revision,NEW.created_at,NEW.created_by_id, NEW.data-'print_requests'-'sent_date'-'channel'-'dispatch_reference'-'sent_recorded_at'-'sent_by_id') IS DISTINCT FROM ROW(OLD.id,OLD.project_id,OLD.bo_id,OLD.group_id,OLD.vendor_id,OLD.scope_revision,OLD.created_at,OLD.created_by_id,OLD.data-'print_requests'-'sent_date'-'channel'-'dispatch_reference'-'sent_recorded_at'-'sent_by_id') THEN RAISE EXCEPTION 'Generated vendor indent scope is immutable'; END IF;
IF OLD.data->>'sent_date' IS NOT NULL AND ROW(NEW.data->'sent_date',NEW.data->'channel',NEW.data->'dispatch_reference',NEW.data->'sent_recorded_at',NEW.data->'sent_by_id') IS DISTINCT FROM ROW(OLD.data->'sent_date',OLD.data->'channel',OLD.data->'dispatch_reference',OLD.data->'sent_recorded_at',OLD.data->'sent_by_id') THEN RAISE EXCEPTION 'Recorded dispatch is immutable';END IF;
RETURN NEW; END $$;

CREATE FUNCTION public.protect_indent_group_evidence() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Grouped indent evidence cannot be deleted or rewritten'; END $$;

CREATE FUNCTION public.protect_indent_group_owner() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN IF ROW(NEW.bo_id,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.bo_id,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Grouped indent identity and creation evidence are immutable'; END IF;RETURN NEW; END $$;

CREATE FUNCTION public.protect_indent_group_project() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN IF NEW.project_id IS DISTINCT FROM OLD.project_id THEN RAISE EXCEPTION 'Grouped indent project ownership is immutable';END IF;RETURN NEW;END $$;

CREATE FUNCTION public.protect_indent_notice_owner() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN IF ROW(NEW.id,NEW.document_id,NEW.recipient_id,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.id,OLD.document_id,OLD.recipient_id,OLD.created_at,OLD.created_by_id) OR OLD.resolved_at IS NOT NULL THEN RAISE EXCEPTION 'Dispatch notice identity and resolution are immutable';END IF;RETURN NEW;END $$;

CREATE FUNCTION public.protect_manufacturer_evidence() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Manufacturer audit and import evidence is immutable'; END; $$;

CREATE FUNCTION public.protect_organisation_director_evidence() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Director designation evidence cannot be deleted or rewritten';END $$;

CREATE FUNCTION public.protect_organisation_director_identity() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF ROW(NEW.slot,NEW.id,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.slot,OLD.id,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Director designation identity is immutable';END IF;
 IF NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'Director designation requires the next version';END IF;RETURN NEW;END $$;

CREATE FUNCTION public.protect_package_file() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Package source cannot be deleted'; END IF;
 IF OLD.state='ready' OR ROW(NEW.id,NEW.receipt_id,NEW.client_file_key,NEW.filename,NEW.kind,NEW.declared_size,NEW.declared_sha256,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.id,OLD.receipt_id,OLD.client_file_key,OLD.filename,OLD.kind,OLD.declared_size,OLD.declared_sha256,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Package source identity is immutable'; END IF;RETURN NEW; END $$;

CREATE FUNCTION public.protect_project_budget_child_identity() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF ROW(NEW.id,NEW.project_id,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.id,OLD.project_id,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Budget child identity is immutable';END IF;RETURN NEW;END $$;

CREATE FUNCTION public.protect_project_budget_evidence() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Budget evidence is immutable; deactivate records instead';END $$;

CREATE FUNCTION public.protect_project_budget_identity() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF ROW(NEW.project_id,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.project_id,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Budget identity is immutable';END IF;
 IF NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'Budget requires the next version';END IF;RETURN NEW;END $$;

CREATE FUNCTION public.protect_project_budget_item_scope() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF TG_OP='UPDATE' AND ROW(NEW.bo_id,NEW.line_id) IS DISTINCT FROM ROW(OLD.bo_id,OLD.line_id) THEN RAISE EXCEPTION 'Allocation source identity is immutable';END IF;
 IF NOT EXISTS(SELECT 1 FROM purchase_bo_lists b JOIN purchase_bo_lines l ON l.bo_id=b.id WHERE b.id=NEW.bo_id AND b.project_id=NEW.project_id AND l.id=NEW.line_id) THEN RAISE EXCEPTION 'Budget allocation belongs to a different project or BO';END IF;RETURN NEW;END $$;

CREATE FUNCTION public.protect_project_material_history() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Project material history is immutable; deactivate instead'; END $$;

CREATE FUNCTION public.protect_project_material_identity() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF ROW(NEW.project_id,NEW.item_id,NEW.engineering_key,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.project_id,OLD.item_id,OLD.engineering_key,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Project material identity cannot be replaced'; END IF;RETURN NEW;
END $$;

CREATE FUNCTION public.protect_project_responsibility_evidence() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Project responsibility evidence is immutable; deactivate instead'; END $$;

CREATE FUNCTION public.protect_project_responsibility_scope() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF ROW(NEW.id,NEW.project_id,NEW.user_id,NEW.bundle_id,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.id,OLD.project_id,OLD.user_id,OLD.bundle_id,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Project responsibility identity is immutable';END IF;
 IF NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'Project responsibility requires the next version';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.protect_purchase_acceptance() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE o purchase_orders; expected jsonb; response jsonb;BEGIN
 SELECT * INTO o FROM purchase_orders WHERE id=NEW.order_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=NEW.order_version AND state='released' AND is_active;
 IF o.id IS NULL THEN RAISE EXCEPTION 'Acceptance requires its actual released project PO';END IF;
 expected=jsonb_build_object('id',o.id,'project_id',o.project_id,'package_id',o.package_id,'version',o.version,'number',o.number,
 'date',o.data->'details'->>'date','supplier_id',o.source_snapshot->'supplier'->>'id','supplier_name',o.source_snapshot->'supplier'->>'name',
 'description',o.source_snapshot->>'description','quantity',o.source_snapshot->>'quantity','uom',o.source_snapshot->>'uom',
 'currency',o.source_snapshot->>'currency','total',o.source_snapshot->>'total','payment_terms',o.source_snapshot->>'payment_terms',
 'delivery_days',o.source_snapshot->'delivery_days','warranty',o.source_snapshot->>'warranty','released_at',o.data->'released_at','released_by_id',o.data->>'released_by_id');
 IF NEW.order_snapshot IS DISTINCT FROM expected THEN RAISE EXCEPTION 'Acceptance source must equal the immutable released PO';END IF;
 IF NEW.data->>'id' IS DISTINCT FROM NEW.id::text OR NEW.data->>'project_id' IS DISTINCT FROM NEW.project_id::text OR NEW.data->>'package_id' IS DISTINCT FROM NEW.package_id::text OR NEW.data->>'version' IS DISTINCT FROM NEW.version::text OR NEW.data->>'is_active' IS DISTINCT FROM NEW.is_active::text OR NEW.data->'order_snapshot' IS DISTINCT FROM NEW.order_snapshot OR NEW.data->'response'->>'status' IS DISTINCT FROM NEW.status THEN RAISE EXCEPTION 'Acceptance projection mismatch';END IF;
 response=NEW.data->'response';
 IF response->>'status'='pending' AND coalesce(response->>'accepted_date','')<>'' THEN RAISE EXCEPTION 'Pending response has no accepted date';END IF;
 IF response->>'status'<>'pending' AND (coalesce(response->>'accepted_date','')='' OR (response->>'accepted_date')::date<(o.data->'details'->>'date')::date) THEN RAISE EXCEPTION 'Acceptance requires a date on or after its PO';END IF;
 IF response->>'status'='accepted_with_exceptions' AND length(trim(coalesce(response->>'remarks','')))=0 THEN RAISE EXCEPTION 'Acceptance exceptions must be recorded';END IF;
 IF TG_OP='INSERT' AND NEW.version<>1 THEN RAISE EXCEPTION 'Acceptance begins at version one';END IF;
 IF TG_OP='UPDATE' AND (NEW.id<>OLD.id OR NEW.project_id<>OLD.project_id OR NEW.package_id<>OLD.package_id OR NEW.order_id<>OLD.order_id OR NEW.order_version<>OLD.order_version OR NEW.order_snapshot IS DISTINCT FROM OLD.order_snapshot OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1) THEN RAISE EXCEPTION 'Acceptance ownership/source/history must be retained';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.protect_purchase_acceptance_history() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Acceptance records and history cannot be erased';END $$;

CREATE FUNCTION public.protect_purchase_advance() RETURNS trigger
    LANGUAGE plpgsql
    AS $_$
DECLARE o purchase_orders;a purchase_acceptances;d jsonb;s jsonb;expected jsonb; pct numeric;calculated numeric;
BEGIN
 d=NEW.data;s=NEW.source_snapshot;
 SELECT * INTO o FROM purchase_orders WHERE id=NEW.order_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=NEW.order_version AND state='released' AND is_active;
 SELECT * INTO a FROM purchase_acceptances WHERE id=(s->>'acceptance_id')::uuid AND project_id=NEW.project_id AND package_id=NEW.package_id AND order_id=NEW.order_id AND version=(s->>'acceptance_version')::integer AND status='accepted' AND is_active;
 IF o.id IS NULL OR a.id IS NULL THEN RAISE EXCEPTION 'Advance requires its actual released and unconditionally accepted PO';END IF;
 expected=jsonb_build_object('project_id',o.project_id,'package_id',o.package_id,'order_id',o.id,'order_version',o.version,'number',o.number,'date',o.data->'details'->>'date',
 'supplier_id',o.source_snapshot->'supplier'->>'id','supplier_name',o.source_snapshot->'supplier'->>'name','currency',o.source_snapshot->>'currency','total',o.source_snapshot->>'total','payment_terms',o.source_snapshot->>'payment_terms',
 'acceptance_id',a.id,'acceptance_version',a.version,'calculation_basis','po_total');
 IF s-ARRAY['supplier_verification','available','blockers'] IS DISTINCT FROM expected OR s->>'available' IS DISTINCT FROM 'true' OR s->'blockers' IS DISTINCT FROM '[]'::jsonb OR coalesce(s->>'supplier_verification','')='' THEN RAISE EXCEPTION 'Advance source projection mismatch';END IF;
 IF d->>'id' IS DISTINCT FROM NEW.id::text OR d->>'project_id' IS DISTINCT FROM NEW.project_id::text OR d->>'package_id' IS DISTINCT FROM NEW.package_id::text OR d->>'version' IS DISTINCT FROM NEW.version::text OR d->>'state' IS DISTINCT FROM NEW.state OR d->'source_snapshot' IS DISTINCT FROM s OR d->>'is_active' IS DISTINCT FROM NEW.is_active::text OR d->>'created_by_id' IS DISTINCT FROM NEW.created_by_id::text OR d->>'updated_by_id' IS DISTINCT FROM NEW.updated_by_id::text OR (d->>'created_at')::timestamptz IS DISTINCT FROM NEW.created_at OR (d->>'updated_at')::timestamptz IS DISTINCT FROM NEW.updated_at OR nullif(d->>'decision_id','')::uuid IS DISTINCT FROM NEW.decision_id THEN RAISE EXCEPTION 'Advance record projection mismatch';END IF;
 IF d->'details'->>'kind' NOT IN('payment','not_applicable') OR d->'details'->>'kind' IS NULL THEN RAISE EXCEPTION 'Declare the advance disposition';END IF;
 IF coalesce(d->'details'->>'advance_pct','')<>'' THEN
  IF d->'details'->>'advance_pct' !~ '^(0|[1-9][0-9]{0,13})([.][0-9]{1,2})?$' THEN RAISE EXCEPTION 'Use exact advance percentage';END IF;
  pct=(d->'details'->>'advance_pct')::numeric;calculated=round((s->>'total')::numeric*pct/100,2);
  IF pct>100 OR d->>'amount' IS DISTINCT FROM calculated::text THEN RAISE EXCEPTION 'Advance amount must equal the exact PO percentage';END IF;
 ELSE IF NEW.state<>'draft' OR coalesce(d->>'amount','')<>'' THEN RAISE EXCEPTION 'Submitted advance requires exact amount';END IF;END IF;
 IF d->'details'->>'kind'='not_applicable' AND (coalesce(pct,0)<>0 OR coalesce(d->'details'->>'proforma_invoice_ref','')<>'' OR coalesce(d->'details'->>'proforma_invoice_date','')<>'' OR coalesce(d->'details'->>'bank_account_ref','')<>'') THEN RAISE EXCEPTION 'No-advance disposition carries no payment instructions';END IF;
 IF NEW.state='draft' THEN
  IF NEW.decision_id IS NOT NULL OR coalesce(d->>'submitted_by_id','')<>'' OR d->'submitted_at'<>'null'::jsonb OR coalesce((d->>'submitted_version')::integer,0)<>0 OR coalesce(d->>'reviewed_by_id','')<>'' OR d->'reviewed_at'<>'null'::jsonb OR d->'payment'<>'null'::jsonb OR NEW.submitted_snapshot<>'' OR NEW.source_fingerprint<>'' THEN RAISE EXCEPTION 'Draft retains no effective approval or payment';END IF;
 ELSE
  expected=jsonb_build_object('Source',s-ARRAY['available','blockers'],'Details',d->'details','Amount',d->>'amount','SubmittedBy',d->>'submitted_by_id','SubmittedVersion',(d->>'submitted_version')::integer,'SubmittedAt',d->'submitted_at');
  IF NEW.submitted_snapshot='' OR NEW.submitted_snapshot::jsonb IS DISTINCT FROM expected OR NEW.source_fingerprint IS DISTINCT FROM encode(sha256(convert_to(NEW.submitted_snapshot,'UTF8')),'hex') THEN RAISE EXCEPTION 'Submitted advance fingerprint must match exact saved source';END IF;
  IF coalesce((d->>'submitted_version')::integer,0)<2 OR coalesce(d->>'submitted_by_id','')='' OR d->'submitted_at'='null'::jsonb OR length(trim(coalesce(d->'details'->>'request_no','')))=0 OR coalesce(d->'details'->>'request_date','')='' OR (d->'details'->>'request_date')::date<(s->>'date')::date THEN RAISE EXCEPTION 'Complete actual advance submission required';END IF;
  IF d->'details'->>'kind'='payment' AND (calculated<=0 OR length(trim(coalesce(d->'details'->>'proforma_invoice_ref','')))=0 OR coalesce(d->'details'->>'proforma_invoice_date','')='' OR (d->'details'->>'proforma_invoice_date')::date>(d->'details'->>'request_date')::date OR length(trim(coalesce(d->'details'->>'bank_account_ref','')))=0) THEN RAISE EXCEPTION 'Payment request requires positive amount and PI/bank references';END IF;
  IF d->'details'->>'kind'='not_applicable' AND length(trim(coalesce(d->'details'->>'notes','')))=0 THEN RAISE EXCEPTION 'No advance requires its reason';END IF;
 END IF;
 IF NEW.state='requested' AND (NEW.decision_id IS NOT NULL OR coalesce(d->>'reviewed_by_id','')<>'' OR d->'reviewed_at'<>'null'::jsonb OR d->'payment'<>'null'::jsonb) THEN RAISE EXCEPTION 'Requested advance has no decision/payment';END IF;
 IF NEW.state<>'disbursed' AND d->'payment' IS DISTINCT FROM 'null'::jsonb THEN RAISE EXCEPTION 'Only disbursed advance carries payment evidence';END IF;
 IF NEW.state='disbursed' AND (d->'details'->>'kind'<>'payment' OR length(trim(coalesce(d->'payment'->>'voucher_ref','')))=0 OR coalesce(d->'payment'->>'disbursed_date','')='' OR (d->'payment'->>'disbursed_date')::date<((d->>'reviewed_at')::timestamptz AT TIME ZONE 'Asia/Kolkata')::date) THEN
  RAISE EXCEPTION 'Payment requires its actual voucher and approval date';END IF;
 IF TG_OP='INSERT' THEN
  IF NEW.version<>1 OR NEW.state<>'draft' OR NEW.created_at<>NEW.updated_at OR NEW.created_by_id<>NEW.updated_by_id THEN RAISE EXCEPTION 'Advance starts as an attributed draft';END IF;
 ELSE
  IF ROW(NEW.id,NEW.project_id,NEW.package_id,NEW.order_id,NEW.order_version,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.id,OLD.project_id,OLD.package_id,OLD.order_id,OLD.order_version,OLD.created_at,OLD.created_by_id) OR NEW.version<>OLD.version+1 OR NEW.updated_at<OLD.updated_at OR OLD.state='disbursed' THEN RAISE EXCEPTION 'Advance identity/history and completed payment are immutable';END IF;
  IF NOT (OLD.state='draft' AND NEW.state IN('draft','requested') OR OLD.state='requested' AND NEW.state IN('approved','rejected','not_applicable','draft') OR OLD.state IN('approved','rejected','not_applicable') AND NEW.state='draft' OR OLD.state='approved' AND NEW.state='disbursed') THEN RAISE EXCEPTION 'Invalid advance transition';END IF;
  IF NEW.state<>'draft' AND OLD.state<>'draft' AND (NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot OR NEW.submitted_snapshot IS DISTINCT FROM OLD.submitted_snapshot OR NEW.source_fingerprint IS DISTINCT FROM OLD.source_fingerprint) THEN RAISE EXCEPTION 'Submitted source cannot be changed during decision/payment';END IF;
  IF OLD.state='draft' AND NEW.state='requested' AND ((d->>'submitted_version')::integer<>NEW.version OR d->>'submitted_by_id' IS DISTINCT FROM NEW.updated_by_id::text OR (d->>'submitted_at')::timestamptz IS DISTINCT FROM NEW.updated_at) THEN RAISE EXCEPTION 'Submission binds current version and actor';END IF;
  IF OLD.state='approved' AND NEW.state='disbursed' AND (NEW.decision_id IS DISTINCT FROM OLD.decision_id OR d->'reviewed_at' IS DISTINCT FROM OLD.data->'reviewed_at' OR d->'reviewed_by_id' IS DISTINCT FROM OLD.data->'reviewed_by_id') THEN RAISE EXCEPTION 'Payment retains its exact Finance decision';END IF;
 END IF;
 RETURN NEW;END $_$;

CREATE FUNCTION public.protect_purchase_advance_history() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Advance records and history cannot be erased';END $$;

CREATE FUNCTION public.protect_purchase_bo_history() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'BO evidence is immutable; use controlled revisions'; END $$;

CREATE FUNCTION public.protect_purchase_bo_line_owner() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF ROW(NEW.bo_id,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.bo_id,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'BO line ownership/creation evidence is immutable'; END IF;RETURN NEW;
END $$;

CREATE FUNCTION public.protect_purchase_bo_ownership() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF ROW(NEW.project_id,NEW.preparer_id,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.project_id,OLD.preparer_id,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'BO ownership/creation evidence is immutable'; END IF;RETURN NEW;
END $$;

CREATE FUNCTION public.protect_purchase_client_clearance() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE q purchase_quality_cases;r purchase_inspection_reports;i purchase_inspections;d jsonb;expected jsonb;BEGIN
 SELECT * INTO q FROM purchase_quality_cases WHERE id=NEW.quality_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=NEW.quality_version AND is_active;
 IF q.id IS NULL THEN RAISE EXCEPTION 'Client clearance requires current quality evidence';END IF;
 IF (SELECT count(*) FROM purchase_quality_certificates WHERE case_id=q.id)<>5 OR EXISTS(SELECT 1 FROM purchase_quality_certificates WHERE case_id=q.id AND (NOT is_active OR NOT verified OR valid_until IS NOT NULL AND valid_until<(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date)) THEN RAISE EXCEPTION 'All required quality certificates must be verified and valid';END IF;
 SELECT * INTO r FROM purchase_inspection_reports WHERE id=q.report_id AND version=q.report_version AND status='issued' AND is_active;
 IF r.id IS NULL THEN RAISE EXCEPTION 'Client clearance needs the current issued IRN';END IF;
 SELECT * INTO i FROM purchase_inspections WHERE id=r.inspection_id AND version=r.inspection_version AND status='cleared' AND is_active;
 IF i.id IS NULL OR NOT EXISTS(SELECT 1 FROM purchase_acceptances WHERE id=i.acceptance_id AND version=i.acceptance_version AND status='accepted' AND is_active) OR NOT EXISTS(SELECT 1 FROM purchase_technical_cases WHERE id=i.technical_id AND version=i.technical_version AND state IN('approved','approved_with_comments') AND is_active) THEN RAISE EXCEPTION 'Client clearance source evidence is no longer current';END IF;
 expected=q.source_snapshot||jsonb_build_object('quality_id',q.id,'quality_version',q.version,'quality_revision',q.revision);
 IF NEW.source_snapshot IS DISTINCT FROM expected THEN RAISE EXCEPTION 'Client clearance source must equal its current eligible quality and IRN';END IF;
 IF NEW.data->>'id' IS DISTINCT FROM NEW.id::text OR NEW.data->>'project_id' IS DISTINCT FROM NEW.project_id::text OR NEW.data->>'package_id' IS DISTINCT FROM NEW.package_id::text OR NEW.data->>'version' IS DISTINCT FROM NEW.version::text OR NEW.data->>'revision' IS DISTINCT FROM NEW.revision::text OR NEW.data->>'is_active' IS DISTINCT FROM NEW.is_active::text OR NEW.data->'source_snapshot' IS DISTINCT FROM NEW.source_snapshot OR NEW.data->'clearance'->>'status' IS DISTINCT FROM NEW.status OR (NEW.data->>'created_at')::timestamptz IS DISTINCT FROM NEW.created_at OR (NEW.data->>'updated_at')::timestamptz IS DISTINCT FROM NEW.updated_at OR nullif(NEW.data->>'created_by_id','')::uuid IS DISTINCT FROM NEW.created_by_id OR nullif(NEW.data->>'updated_by_id','')::uuid IS DISTINCT FROM NEW.updated_by_id THEN RAISE EXCEPTION 'Client clearance projection mismatch';END IF;
 IF NEW.updated_by_id IS NULL OR NEW.created_by_id IS NULL THEN RAISE EXCEPTION 'Use authenticated client-clearance attribution';END IF;
 d=NEW.data->'clearance';
 IF NEW.status='cleared' AND (length(trim(coalesce(d->>'mdcc_ref','')))=0 OR coalesce(d->>'clearance_date','')='') THEN RAISE EXCEPTION 'Cleared MDCC needs reference and actual clearance date';END IF;
 IF length(coalesce(d->>'mdcc_ref',''))>200 OR length(coalesce(d->>'cleared_by_authority',''))>300 OR length(coalesce(d->>'remarks',''))>4000 OR length(coalesce(d->>'file_name',''))>500 THEN RAISE EXCEPTION 'Client clearance fields exceed limits';END IF;
 IF coalesce(d->>'clearance_date','')<>'' AND ((d->>'clearance_date')::date<(r.data->'report'->>'issue_date')::date OR (d->>'clearance_date')::date>(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date) THEN RAISE EXCEPTION 'Client clearance date follows issued IRN and cannot be future';END IF;
 IF TG_OP='INSERT' AND (NEW.version<>1 OR NEW.revision<>1) THEN RAISE EXCEPTION 'Client clearance begins at version/revision one';END IF;
 IF TG_OP='UPDATE' THEN
 IF NEW.id<>OLD.id OR NEW.project_id<>OLD.project_id OR NEW.package_id<>OLD.package_id OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'Client clearance ownership/audit must be retained';END IF;
 IF NEW.revision=OLD.revision THEN
 IF OLD.status='cleared' OR NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot THEN RAISE EXCEPTION 'Cleared or obsolete MDCC requires revision';END IF;
 ELSIF NEW.revision<>OLD.revision+1 OR NEW.status<>'pending' OR d IS DISTINCT FROM jsonb_build_object('status','pending','mdcc_ref','','clearance_date','','cleared_by_authority','','remarks','','file_name','') THEN RAISE EXCEPTION 'MDCC revision resets clearance and retains prior evidence';END IF;
 END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.protect_purchase_client_clearance_history() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'IR / IRN and history cannot be erased';END $$;

CREATE FUNCTION public.protect_purchase_dispatch() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE c purchase_client_clearances;q purchase_quality_cases;r purchase_inspection_reports;i purchase_inspections;d jsonb;expected jsonb;BEGIN
 SELECT * INTO c FROM purchase_client_clearances WHERE id=NEW.clearance_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=NEW.clearance_version AND status='cleared' AND is_active;
 IF c.id IS NULL THEN RAISE EXCEPTION 'Dispatch needs its current cleared client MDCC';END IF;
 SELECT * INTO q FROM purchase_quality_cases WHERE id=c.quality_id AND version=c.quality_version AND is_active;
 IF q.id IS NULL OR (SELECT count(*) FROM purchase_quality_certificates WHERE case_id=q.id)<>5 OR EXISTS(SELECT 1 FROM purchase_quality_certificates WHERE case_id=q.id AND (NOT is_active OR NOT verified OR valid_until IS NOT NULL AND valid_until<(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date)) THEN RAISE EXCEPTION 'Dispatch requires current verified quality';END IF;
 SELECT * INTO r FROM purchase_inspection_reports WHERE id=q.report_id AND version=q.report_version AND status='issued' AND is_active;
 IF r.id IS NULL THEN RAISE EXCEPTION 'Dispatch needs current issued IRN';END IF;
 SELECT * INTO i FROM purchase_inspections WHERE id=r.inspection_id AND version=r.inspection_version AND status='cleared' AND is_active;
 IF i.id IS NULL OR NOT EXISTS(SELECT 1 FROM purchase_acceptances WHERE id=i.acceptance_id AND version=i.acceptance_version AND status='accepted' AND is_active) OR NOT EXISTS(SELECT 1 FROM purchase_technical_cases WHERE id=i.technical_id AND version=i.technical_version AND state IN('approved','approved_with_comments') AND is_active) THEN RAISE EXCEPTION 'Dispatch source evidence is no longer current';END IF;
 expected=c.source_snapshot||jsonb_build_object('clearance_id',c.id,'clearance_version',c.version,'clearance_revision',c.revision,'mdcc_ref',c.data->'clearance'->>'mdcc_ref','clearance_date',c.data->'clearance'->>'clearance_date');
 IF NEW.source_snapshot IS DISTINCT FROM expected THEN RAISE EXCEPTION 'Dispatch source must equal its current client clearance';END IF;
 IF NEW.data->>'id' IS DISTINCT FROM NEW.id::text OR NEW.data->>'project_id' IS DISTINCT FROM NEW.project_id::text OR NEW.data->>'package_id' IS DISTINCT FROM NEW.package_id::text OR NEW.data->>'version' IS DISTINCT FROM NEW.version::text OR NEW.data->>'revision' IS DISTINCT FROM NEW.revision::text OR NEW.data->>'is_active' IS DISTINCT FROM NEW.is_active::text OR NEW.data->'source_snapshot' IS DISTINCT FROM NEW.source_snapshot OR NEW.data->'dispatch'->>'status' IS DISTINCT FROM NEW.status OR (NEW.data->>'created_at')::timestamptz IS DISTINCT FROM NEW.created_at OR (NEW.data->>'updated_at')::timestamptz IS DISTINCT FROM NEW.updated_at OR nullif(NEW.data->>'created_by_id','')::uuid IS DISTINCT FROM NEW.created_by_id OR nullif(NEW.data->>'updated_by_id','')::uuid IS DISTINCT FROM NEW.updated_by_id THEN RAISE EXCEPTION 'Dispatch projection mismatch';END IF;
 IF NEW.created_by_id IS NULL OR NEW.updated_by_id IS NULL THEN RAISE EXCEPTION 'Use authenticated dispatch attribution';END IF;
 d=NEW.data->'dispatch';
 IF NEW.status='dispatched' AND (length(trim(coalesce(d->>'transporter_name','')))=0 OR length(trim(coalesce(d->>'lr_number','')))=0 OR coalesce(d->>'dispatch_date','')='') THEN RAISE EXCEPTION 'Dispatched needs transporter, LR and actual date';END IF;
 IF length(coalesce(d->>'transporter_name',''))>300 OR length(coalesce(d->>'lr_number',''))>200 OR length(coalesce(d->>'vehicle_number',''))>100 OR length(coalesce(d->>'eway_bill_number',''))>200 OR length(coalesce(d->>'invoice_number',''))>200 OR length(coalesce(d->>'driver_contact',''))>100 OR length(coalesce(d->>'file_name',''))>500 THEN RAISE EXCEPTION 'Dispatch fields exceed limits';END IF;
 IF coalesce(d->>'dispatch_date','')<>'' AND ((d->>'dispatch_date')::date<(c.data->'clearance'->>'clearance_date')::date OR (d->>'dispatch_date')::date>(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date) THEN RAISE EXCEPTION 'Dispatch date follows client clearance and cannot be future';END IF;
 IF coalesce(d->>'lr_date','')<>'' AND (d->>'lr_date')::date>(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date OR coalesce(d->>'invoice_date','')<>'' AND (d->>'invoice_date')::date>(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date THEN RAISE EXCEPTION 'Use nonfuture actual logistics document dates';END IF;
 IF coalesce(d->>'eway_bill_valid_until','')<>'' THEN PERFORM (d->>'eway_bill_valid_until')::date;END IF;
 IF TG_OP='INSERT' AND (NEW.version<>1 OR NEW.revision<>1) THEN RAISE EXCEPTION 'Dispatch begins at version/revision one';END IF;
 IF TG_OP='UPDATE' THEN
 IF NEW.id<>OLD.id OR NEW.project_id<>OLD.project_id OR NEW.package_id<>OLD.package_id OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'Dispatch ownership/audit must be retained';END IF;
 IF NEW.revision=OLD.revision THEN
 IF OLD.status='dispatched' OR NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot THEN RAISE EXCEPTION 'Dispatched or obsolete logistics needs revision';END IF;
 ELSIF NEW.revision<>OLD.revision+1 OR NEW.status<>'pending' OR d IS DISTINCT FROM jsonb_build_object('status','pending','dispatch_date','','transporter_name','','lr_number','','lr_date','','vehicle_number','','eway_bill_number','','eway_bill_valid_until','','invoice_number','','invoice_date','','driver_contact','','file_name','') THEN RAISE EXCEPTION 'Dispatch revision resets pending and retains prior evidence';END IF;
 END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.protect_purchase_dispatch_history() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'IR / IRN and history cannot be erased';END $$;

CREATE FUNCTION public.protect_purchase_engineer_confirmations() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE z purchase_grns;y purchase_site_receipts;x purchase_dispatches;c purchase_client_clearances;q purchase_quality_cases;r purchase_inspection_reports;i purchase_inspections;d jsonb;expected jsonb;BEGIN
 SELECT * INTO z FROM purchase_grns WHERE id=NEW.grn_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=NEW.grn_version AND status='issued' AND is_active;
 IF z.id IS NULL THEN RAISE EXCEPTION 'Engineer confirmation needs its current issued GRN';END IF;
 SELECT * INTO y FROM purchase_site_receipts WHERE id=z.receipt_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=z.receipt_version AND status='delivered_at_site' AND is_active;
 IF y.id IS NULL THEN RAISE EXCEPTION 'Engineer confirmation needs the current delivered site receipt';END IF;
 SELECT * INTO x FROM purchase_dispatches WHERE id=y.dispatch_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=y.dispatch_version AND status='dispatched' AND is_active;
 IF x.id IS NULL THEN RAISE EXCEPTION 'GRN needs its current dispatched material';END IF;
 SELECT * INTO c FROM purchase_client_clearances WHERE id=x.clearance_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=x.clearance_version AND status='cleared' AND is_active;
 IF c.id IS NULL THEN RAISE EXCEPTION 'GRN needs its current cleared client MDCC';END IF;
 SELECT * INTO q FROM purchase_quality_cases WHERE id=c.quality_id AND version=c.quality_version AND is_active;
 IF q.id IS NULL OR (SELECT count(*) FROM purchase_quality_certificates WHERE case_id=q.id)<>5 OR EXISTS(SELECT 1 FROM purchase_quality_certificates WHERE case_id=q.id AND (NOT is_active OR NOT verified OR valid_until IS NOT NULL AND valid_until<(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date)) THEN RAISE EXCEPTION 'GRN requires current verified quality';END IF;
 SELECT * INTO r FROM purchase_inspection_reports WHERE id=q.report_id AND version=q.report_version AND status='issued' AND is_active;
 IF r.id IS NULL THEN RAISE EXCEPTION 'GRN needs current issued IRN';END IF;
 SELECT * INTO i FROM purchase_inspections WHERE id=r.inspection_id AND version=r.inspection_version AND status='cleared' AND is_active;
 IF i.id IS NULL OR NOT EXISTS(SELECT 1 FROM purchase_acceptances WHERE id=i.acceptance_id AND version=i.acceptance_version AND status='accepted' AND is_active) OR NOT EXISTS(SELECT 1 FROM purchase_technical_cases WHERE id=i.technical_id AND version=i.technical_version AND state IN('approved','approved_with_comments') AND is_active) THEN RAISE EXCEPTION 'GRN source evidence is no longer current';END IF;
 expected=z.source_snapshot||jsonb_build_object('grn_id',z.id,'grn_version',z.version,'grn_revision',z.revision,'grn_number',z.data->'grn'->>'grn_number','grn_date',z.data->'grn'->>'grn_date','received_qty',z.data->'grn'->>'received_qty','accepted_qty',z.data->'grn'->>'accepted_qty','rejected_qty',z.data->'grn'->>'rejected_qty','warranty_ref',z.data->'grn'->>'warranty_ref','warranty_valid_until',z.data->'grn'->>'warranty_valid_until');
 IF NEW.source_snapshot IS DISTINCT FROM expected THEN RAISE EXCEPTION 'Engineer confirmation source must equal its current issued GRN';END IF;
 IF NEW.data->>'id' IS DISTINCT FROM NEW.id::text OR NEW.data->>'project_id' IS DISTINCT FROM NEW.project_id::text OR NEW.data->>'package_id' IS DISTINCT FROM NEW.package_id::text OR NEW.data->>'version' IS DISTINCT FROM NEW.version::text OR NEW.data->>'revision' IS DISTINCT FROM NEW.revision::text OR NEW.data->>'is_active' IS DISTINCT FROM NEW.is_active::text OR NEW.data->'source_snapshot' IS DISTINCT FROM NEW.source_snapshot OR NEW.data->'confirmation'->>'status' IS DISTINCT FROM NEW.status OR (NEW.data->>'created_at')::timestamptz IS DISTINCT FROM NEW.created_at OR (NEW.data->>'updated_at')::timestamptz IS DISTINCT FROM NEW.updated_at OR nullif(NEW.data->>'created_by_id','')::uuid IS DISTINCT FROM NEW.created_by_id OR nullif(NEW.data->>'updated_by_id','')::uuid IS DISTINCT FROM NEW.updated_by_id THEN RAISE EXCEPTION 'Engineer confirmation projection mismatch';END IF;
 IF NEW.created_by_id IS NULL OR NEW.updated_by_id IS NULL THEN RAISE EXCEPTION 'Use authenticated engineer confirmation attribution';END IF;
 d=NEW.data->'confirmation';
 IF NEW.status='confirmed' AND (length(trim(coalesce(d->>'engineer_name','')))=0 OR coalesce(d->>'confirmation_date','')='') THEN RAISE EXCEPTION 'Confirmed needs actual engineer and date';END IF;
 IF length(coalesce(d->>'engineer_name',''))>300 OR length(coalesce(d->>'designation',''))>200 OR length(coalesce(d->>'mbook_ref',''))>200 OR length(coalesce(d->>'work_package_ref',''))>200 OR length(coalesce(d->>'remarks',''))>4000 THEN RAISE EXCEPTION 'Engineer confirmation fields exceed limits';END IF;
 IF coalesce(d->>'confirmation_date','')<>'' AND ((d->>'confirmation_date')::date<(z.data->'grn'->>'grn_date')::date OR (d->>'confirmation_date')::date>(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date) THEN RAISE EXCEPTION 'Confirmation follows actual GRN and cannot be future';END IF;
 IF TG_OP='INSERT' AND (NEW.version<>1 OR NEW.revision<>1) THEN RAISE EXCEPTION 'Engineer confirmation begins at version/revision one';END IF;
 IF TG_OP='UPDATE' THEN
 IF NEW.id<>OLD.id OR NEW.project_id<>OLD.project_id OR NEW.package_id<>OLD.package_id OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'Engineer confirmation ownership/audit must be retained';END IF;
 IF NEW.revision=OLD.revision THEN
 IF OLD.status='confirmed' OR NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot THEN RAISE EXCEPTION 'Confirmed or obsolete engineer confirmation needs revision';END IF;
 ELSIF NEW.revision<>OLD.revision+1 OR NEW.status<>'pending' OR d IS DISTINCT FROM jsonb_build_object('status','pending','confirmation_date','','engineer_name','','designation','','mbook_ref','','work_package_ref','','remarks','') THEN RAISE EXCEPTION 'Engineer confirmation revision resets pending and retains prior evidence';END IF;
 END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.protect_purchase_engineer_confirmations_history() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Engineer confirmation and history cannot be erased';END $$;

CREATE FUNCTION public.protect_purchase_grns() RETURNS trigger
    LANGUAGE plpgsql
    AS $_$ DECLARE y purchase_site_receipts;x purchase_dispatches;c purchase_client_clearances;q purchase_quality_cases;r purchase_inspection_reports;i purchase_inspections;d jsonb;expected jsonb;BEGIN
 SELECT * INTO y FROM purchase_site_receipts WHERE id=NEW.receipt_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=NEW.receipt_version AND status='delivered_at_site' AND is_active;
 IF y.id IS NULL THEN RAISE EXCEPTION 'GRN needs its current delivered GRN';END IF;
 SELECT * INTO x FROM purchase_dispatches WHERE id=y.dispatch_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=y.dispatch_version AND status='dispatched' AND is_active;
 IF x.id IS NULL THEN RAISE EXCEPTION 'GRN needs its current dispatched material';END IF;
 SELECT * INTO c FROM purchase_client_clearances WHERE id=x.clearance_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=x.clearance_version AND status='cleared' AND is_active;
 IF c.id IS NULL THEN RAISE EXCEPTION 'GRN needs its current cleared client MDCC';END IF;
 SELECT * INTO q FROM purchase_quality_cases WHERE id=c.quality_id AND version=c.quality_version AND is_active;
 IF q.id IS NULL OR (SELECT count(*) FROM purchase_quality_certificates WHERE case_id=q.id)<>5 OR EXISTS(SELECT 1 FROM purchase_quality_certificates WHERE case_id=q.id AND (NOT is_active OR NOT verified OR valid_until IS NOT NULL AND valid_until<(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date)) THEN RAISE EXCEPTION 'GRN requires current verified quality';END IF;
 SELECT * INTO r FROM purchase_inspection_reports WHERE id=q.report_id AND version=q.report_version AND status='issued' AND is_active;
 IF r.id IS NULL THEN RAISE EXCEPTION 'GRN needs current issued IRN';END IF;
 SELECT * INTO i FROM purchase_inspections WHERE id=r.inspection_id AND version=r.inspection_version AND status='cleared' AND is_active;
 IF i.id IS NULL OR NOT EXISTS(SELECT 1 FROM purchase_acceptances WHERE id=i.acceptance_id AND version=i.acceptance_version AND status='accepted' AND is_active) OR NOT EXISTS(SELECT 1 FROM purchase_technical_cases WHERE id=i.technical_id AND version=i.technical_version AND state IN('approved','approved_with_comments') AND is_active) THEN RAISE EXCEPTION 'GRN source evidence is no longer current';END IF;
 expected=y.source_snapshot||jsonb_build_object('receipt_id',y.id,'receipt_version',y.version,'receipt_revision',y.revision,'arrival_date',y.data->'receipt'->>'arrival_date','receiving_officer',y.data->'receipt'->>'receiving_officer','packages_received',y.data->'receipt'->'packages_received','packages_condition',y.data->'receipt'->>'packages_condition','transit_damage_notes',y.data->'receipt'->>'transit_damage_notes','gate_entry_ref',y.data->'receipt'->>'gate_entry_ref');
 IF NEW.source_snapshot IS DISTINCT FROM expected THEN RAISE EXCEPTION 'GRN source must equal its current site receipt';END IF;
 IF NEW.data->>'id' IS DISTINCT FROM NEW.id::text OR NEW.data->>'project_id' IS DISTINCT FROM NEW.project_id::text OR NEW.data->>'package_id' IS DISTINCT FROM NEW.package_id::text OR NEW.data->>'version' IS DISTINCT FROM NEW.version::text OR NEW.data->>'revision' IS DISTINCT FROM NEW.revision::text OR NEW.data->>'is_active' IS DISTINCT FROM NEW.is_active::text OR NEW.data->'source_snapshot' IS DISTINCT FROM NEW.source_snapshot OR NEW.data->'grn'->>'status' IS DISTINCT FROM NEW.status OR (NEW.data->>'created_at')::timestamptz IS DISTINCT FROM NEW.created_at OR (NEW.data->>'updated_at')::timestamptz IS DISTINCT FROM NEW.updated_at OR nullif(NEW.data->>'created_by_id','')::uuid IS DISTINCT FROM NEW.created_by_id OR nullif(NEW.data->>'updated_by_id','')::uuid IS DISTINCT FROM NEW.updated_by_id THEN RAISE EXCEPTION 'GRN projection mismatch';END IF;
 IF NEW.created_by_id IS NULL OR NEW.updated_by_id IS NULL THEN RAISE EXCEPTION 'Use authenticated receiving attribution';END IF;
 d=NEW.data->'grn';
 IF NEW.status='issued' AND (length(trim(coalesce(d->>'grn_number','')))=0 OR coalesce(d->>'grn_date','')='') THEN RAISE EXCEPTION 'Issued GRN needs number and actual date';END IF;
 IF d->>'uom' IS DISTINCT FROM NEW.source_snapshot->>'uom' THEN RAISE EXCEPTION 'GRN uses inherited material unit';END IF;
 IF jsonb_typeof(d->'received_qty') IS DISTINCT FROM 'string' OR jsonb_typeof(d->'accepted_qty') IS DISTINCT FROM 'string' OR jsonb_typeof(d->'rejected_qty') IS DISTINCT FROM 'string' OR coalesce(d->>'received_qty','') !~ '^(0|[1-9][0-9]{0,11})(\.[0-9]{1,6})?$' OR coalesce(d->>'accepted_qty','') !~ '^(0|[1-9][0-9]{0,11})(\.[0-9]{1,6})?$' OR coalesce(d->>'rejected_qty','') !~ '^(0|[1-9][0-9]{0,11})(\.[0-9]{1,6})?$' THEN RAISE EXCEPTION 'Use exact nonnegative GRN decimal quantities';END IF;
 IF NEW.status='issued' AND ((d->>'received_qty')::numeric<=0 OR (d->>'received_qty')::numeric>(NEW.source_snapshot->>'cleared_quantity')::numeric OR (d->>'accepted_qty')::numeric+(d->>'rejected_qty')::numeric>(d->>'received_qty')::numeric) THEN RAISE EXCEPTION 'Issued GRN breakdown must fit actual received and cleared allowance';END IF;
 IF length(coalesce(d->>'grn_number',''))>200 OR length(coalesce(d->>'store_keeper_name',''))>300 OR length(coalesce(d->>'remarks',''))>4000 OR length(coalesce(d->>'warranty_ref',''))>200 OR length(coalesce(d->>'file_name',''))>500 THEN RAISE EXCEPTION 'GRN fields exceed limits';END IF;
 IF coalesce(d->>'grn_date','')<>'' AND ((d->>'grn_date')::date<(y.data->'receipt'->>'arrival_date')::date OR (d->>'grn_date')::date>(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date) THEN RAISE EXCEPTION 'GRN follows actual arrival and cannot be future';END IF;
 IF coalesce(d->>'warranty_valid_until','')<>'' THEN PERFORM (d->>'warranty_valid_until')::date;END IF;
 IF TG_OP='INSERT' AND (NEW.version<>1 OR NEW.revision<>1) THEN RAISE EXCEPTION 'GRN begins at version/revision one';END IF;
 IF TG_OP='UPDATE' THEN
 IF NEW.id<>OLD.id OR NEW.project_id<>OLD.project_id OR NEW.package_id<>OLD.package_id OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'GRN ownership/audit must be retained';END IF;
 IF NEW.revision=OLD.revision THEN
 IF OLD.status='issued' OR NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot THEN RAISE EXCEPTION 'Issued or obsolete GRN needs revision';END IF;
 ELSIF NEW.revision<>OLD.revision+1 OR NEW.status<>'pending' OR d IS DISTINCT FROM jsonb_build_object('status','pending','grn_number','','grn_date','','received_qty','0','accepted_qty','0','rejected_qty','0','uom',NEW.source_snapshot->>'uom','store_keeper_name','','remarks','','warranty_ref','','warranty_valid_until','','file_name','') THEN RAISE EXCEPTION 'GRN revision resets pending and retains prior evidence';END IF;
 END IF;
 RETURN NEW;END $_$;

CREATE FUNCTION public.protect_purchase_grns_history() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'GRN and history cannot be erased';END $$;

CREATE FUNCTION public.protect_purchase_hold_history() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Purchase hold evidence is immutable; deactivate instead'; END $$;

CREATE FUNCTION public.protect_purchase_hold_scope() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN IF ROW(NEW.project_id,NEW.doer_id,NEW.resolver_id,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.project_id,OLD.doer_id,OLD.resolver_id,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Hold scope and assignment evidence are immutable'; END IF;RETURN NEW; END $$;

CREATE FUNCTION public.protect_purchase_indent_history() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Purchase indent evidence is immutable; deactivate instead'; END $$;

CREATE FUNCTION public.protect_purchase_indent_scope() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN IF ROW(NEW.project_id,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.project_id,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Indent scope evidence are immutable'; END IF;RETURN NEW; END $$;

CREATE FUNCTION public.protect_purchase_inspection() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE o purchase_orders;a purchase_acceptances;t purchase_technical_cases;expected jsonb;q jsonb;i jsonb;point jsonb;codes text[]='{}';day date;BEGIN
 SELECT * INTO o FROM purchase_orders WHERE id=NEW.order_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=NEW.order_version AND state='released' AND is_active;
 SELECT * INTO a FROM purchase_acceptances WHERE id=NEW.acceptance_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND order_id=NEW.order_id AND order_version=NEW.order_version AND version=NEW.acceptance_version AND status='accepted' AND is_active;
 SELECT * INTO t FROM purchase_technical_cases WHERE id=NEW.technical_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=NEW.technical_version AND state IN('approved','approved_with_comments') AND is_active;
 IF o.id IS NULL OR a.id IS NULL OR t.id IS NULL THEN RAISE EXCEPTION 'Inspection requires its current released PO, unconditional acceptance and approved technical kit';END IF;
 SELECT d INTO q FROM jsonb_array_elements(t.data->'documents') d WHERE d->>'kind'='qap' AND d->>'status'='received' AND d->>'is_active'='true';
 IF q IS NULL OR t.data->'approval'->>'qap_approved' IS DISTINCT FROM 'true' OR t.data->'approval'->>'conditions_closed' IS DISTINCT FROM 'true' OR NOT EXISTS(SELECT 1 FROM purchase_technical_files WHERE id=(q->>'file_id')::uuid AND case_id=t.id AND purpose='document' AND is_active) OR NOT EXISTS(SELECT 1 FROM purchase_technical_files WHERE id=(t.data->'approval'->>'file_id')::uuid AND case_id=t.id AND purpose='client_approval' AND is_active) THEN RAISE EXCEPTION 'Inspection requires actual approved QAP and signed client approval files';END IF;
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

CREATE FUNCTION public.protect_purchase_inspection_history() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Material inspection and history cannot be erased';END $$;

CREATE FUNCTION public.protect_purchase_inspection_report() RETURNS trigger
    LANGUAGE plpgsql
    AS $_$ DECLARE i purchase_inspections;a purchase_acceptances;t purchase_technical_cases;r jsonb;expected jsonb;qty numeric;BEGIN
 SELECT * INTO i FROM purchase_inspections WHERE id=NEW.inspection_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=NEW.inspection_version AND status='cleared' AND is_active;
 IF i.id IS NULL THEN RAISE EXCEPTION 'IR / IRN requires its current cleared inspection';END IF;
 SELECT * INTO a FROM purchase_acceptances WHERE id=i.acceptance_id AND version=i.acceptance_version AND status='accepted' AND is_active;
 SELECT * INTO t FROM purchase_technical_cases WHERE id=i.technical_id AND version=i.technical_version AND state IN('approved','approved_with_comments') AND is_active;
 IF a.id IS NULL OR t.id IS NULL THEN RAISE EXCEPTION 'Inspection report source evidence is no longer current';END IF;
 expected=jsonb_build_object('project_id',NEW.project_id,'package_id',NEW.package_id,'inspection_id',i.id,'inspection_version',i.version,'inspection_revision',i.revision,'actual_date',i.data->'inspection'->>'actual_date','order_id',i.source_snapshot->>'order_id','order_version',i.source_snapshot->'order_version','number',i.source_snapshot->>'number','supplier_name',i.source_snapshot->>'supplier_name','description',i.source_snapshot->>'description','quantity',i.source_snapshot->>'quantity','uom',i.source_snapshot->>'uom','qap_reference',i.source_snapshot->>'qap_reference','available',true,'blockers','[]'::jsonb);
 IF NEW.source_snapshot IS DISTINCT FROM expected THEN RAISE EXCEPTION 'Report source must equal its current inspected material';END IF;
 IF NEW.data->>'id' IS DISTINCT FROM NEW.id::text OR NEW.data->>'project_id' IS DISTINCT FROM NEW.project_id::text OR NEW.data->>'package_id' IS DISTINCT FROM NEW.package_id::text OR NEW.data->>'version' IS DISTINCT FROM NEW.version::text OR NEW.data->>'revision' IS DISTINCT FROM NEW.revision::text OR NEW.data->>'is_active' IS DISTINCT FROM NEW.is_active::text OR NEW.data->'source_snapshot' IS DISTINCT FROM NEW.source_snapshot OR NEW.data->'report'->>'status' IS DISTINCT FROM NEW.status THEN RAISE EXCEPTION 'Inspection report projection mismatch';END IF;
 r=NEW.data->'report';
 IF r->>'uom' IS DISTINCT FROM i.source_snapshot->>'uom' THEN RAISE EXCEPTION 'Report unit is inherited from inspected material';END IF;
 IF coalesce(r->>'cleared_quantity','') !~ '^(0|[1-9][0-9]{0,11})(\.[0-9]{1,6})?$' THEN RAISE EXCEPTION 'Use an exact report quantity';END IF;
 qty=(r->>'cleared_quantity')::numeric;
 IF qty>(i.source_snapshot->>'quantity')::numeric OR NEW.status='issued' AND qty<=0 THEN RAISE EXCEPTION 'Report quantity must match its actual inspected material allowance';END IF;
 IF NEW.status='issued' AND (length(trim(coalesce(r->>'ir_number','')))=0 OR length(trim(coalesce(r->>'irn_number','')))=0 OR coalesce(r->>'issue_date','')='') THEN RAISE EXCEPTION 'Issued report needs IR, IRN and actual issue date';END IF;
 IF length(coalesce(r->>'ir_number',''))>200 OR length(coalesce(r->>'irn_number',''))>200 OR length(coalesce(r->>'issuing_authority',''))>300 OR length(coalesce(r->>'file_name',''))>500 THEN RAISE EXCEPTION 'Report fields exceed their limits';END IF;
 IF coalesce(r->>'issue_date','')<>'' AND ((r->>'issue_date')::date<(i.data->'inspection'->>'actual_date')::date OR (r->>'issue_date')::date>(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date) THEN RAISE EXCEPTION 'Report issue date must follow the actual inspection and cannot be future';END IF;
 IF TG_OP='INSERT' AND (NEW.version<>1 OR NEW.revision<>1) THEN RAISE EXCEPTION 'Report begins at version and revision one';END IF;
 IF TG_OP='UPDATE' THEN
  IF NEW.id<>OLD.id OR NEW.project_id<>OLD.project_id OR NEW.package_id<>OLD.package_id OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'Report ownership and audit must be retained';END IF;
  IF NEW.revision=OLD.revision THEN
   IF OLD.status='issued' OR NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot THEN RAISE EXCEPTION 'Issued or obsolete report requires a revision';END IF;
  ELSIF NEW.revision<>OLD.revision+1 OR NEW.status<>'pending' OR r IS DISTINCT FROM jsonb_build_object('status','pending','ir_number','','irn_number','','issue_date','','issuing_authority','','cleared_quantity','0','uom',i.source_snapshot->>'uom','file_name','') THEN RAISE EXCEPTION 'Report revision resets issuance and retains old evidence';END IF;
 END IF;
 RETURN NEW;END $_$;

CREATE FUNCTION public.protect_purchase_inspection_report_history() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'IR / IRN and history cannot be erased';END $$;

CREATE FUNCTION public.protect_purchase_order() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_sourcing_cases s WHERE s.id=NEW.sourcing_id AND s.project_id=NEW.project_id AND s.package_id=NEW.package_id AND s.state='released' AND s.is_active AND NEW.source_snapshot->>'sourcing_id'=s.id::text AND NEW.source_snapshot->>'sourcing_version'=s.version::text AND NEW.source_snapshot->>'loi_reference'=s.data->>'loi_reference') THEN RAISE EXCEPTION 'Order requires its exact released project LOI';END IF;
 IF NOT EXISTS(SELECT 1 FROM purchase_technical_cases t WHERE t.id=NEW.technical_id AND t.project_id=NEW.project_id AND t.package_id=NEW.package_id AND t.is_active AND t.state IN('approved','approved_with_comments') AND NEW.source_snapshot->'technical'->>'case_id'=t.id::text AND NEW.source_snapshot->'technical'->>'version'=t.version::text AND NEW.source_snapshot->'technical'->>'revision'=t.revision::text) THEN RAISE EXCEPTION 'Order requires its exact approved technical revision';END IF;
 IF NEW.data->>'id' IS DISTINCT FROM NEW.id::text OR NEW.data->>'project_id' IS DISTINCT FROM NEW.project_id::text OR NEW.data->>'package_id' IS DISTINCT FROM NEW.package_id::text OR NEW.data->>'version' IS DISTINCT FROM NEW.version::text OR NEW.data->>'state' IS DISTINCT FROM NEW.state OR NEW.data->>'is_active' IS DISTINCT FROM NEW.is_active::text OR NEW.data->'source_snapshot' IS DISTINCT FROM NEW.source_snapshot OR coalesce(NEW.data->>'number','') IS DISTINCT FROM coalesce(NEW.number,'') THEN RAISE EXCEPTION 'Purchase order state projection mismatch';END IF;
 IF TG_OP='INSERT' AND (NEW.version<>1 OR NEW.state<>'draft') THEN RAISE EXCEPTION 'Orders begin as an owned draft';END IF;
 IF TG_OP='UPDATE' THEN
  IF OLD.state='released' THEN RAISE EXCEPTION 'Released purchase orders are immutable';END IF;
  IF NEW.id<>OLD.id OR NEW.project_id<>OLD.project_id OR NEW.package_id<>OLD.package_id OR NEW.sourcing_id<>OLD.sourcing_id OR NEW.technical_id<>OLD.technical_id OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'Order ownership and audit history must be retained';END IF;
  IF NEW.state='released' AND (NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot OR NEW.data->'details' IS DISTINCT FROM OLD.data->'details' OR NEW.data->>'released_by_id' IS DISTINCT FROM NEW.updated_by_id::text OR NEW.data->>'released_at' IS NULL) THEN RAISE EXCEPTION 'Release freezes prepared terms with authenticated attribution';END IF;
 END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.protect_purchase_order_history() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Purchase orders and history cannot be erased';END $$;

CREATE FUNCTION public.protect_purchase_quality_case() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE r purchase_inspection_reports;i purchase_inspections;expected jsonb;BEGIN
 SELECT * INTO r FROM purchase_inspection_reports WHERE id=NEW.report_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=NEW.report_version AND status='issued' AND is_active;
 IF r.id IS NULL THEN RAISE EXCEPTION 'Quality requires its current issued inspection report';END IF;
 SELECT * INTO i FROM purchase_inspections WHERE id=r.inspection_id AND version=r.inspection_version AND status='cleared' AND is_active;
 IF i.id IS NULL OR NOT EXISTS(SELECT 1 FROM purchase_acceptances WHERE id=i.acceptance_id AND version=i.acceptance_version AND status='accepted' AND is_active) OR NOT EXISTS(SELECT 1 FROM purchase_technical_cases WHERE id=i.technical_id AND version=i.technical_version AND state IN('approved','approved_with_comments') AND is_active) THEN RAISE EXCEPTION 'Quality inspection source evidence is no longer current';END IF;
 expected=jsonb_build_object('project_id',NEW.project_id,'package_id',NEW.package_id,'report_id',r.id,'report_version',r.version,'report_revision',r.revision,'ir_number',r.data->'report'->>'ir_number','irn_number',r.data->'report'->>'irn_number','issue_date',r.data->'report'->>'issue_date','cleared_quantity',r.data->'report'->>'cleared_quantity','uom',r.data->'report'->>'uom','order_id',r.source_snapshot->>'order_id','order_version',r.source_snapshot->'order_version','number',r.source_snapshot->>'number','supplier_name',r.source_snapshot->>'supplier_name','description',r.source_snapshot->>'description','inspection_id',r.inspection_id,'inspection_version',r.inspection_version,'available',true,'blockers','[]'::jsonb);
 IF NEW.source_snapshot IS DISTINCT FROM expected THEN RAISE EXCEPTION 'Quality source must equal its current issued IRN';END IF;
 IF NEW.data->>'id' IS DISTINCT FROM NEW.id::text OR NEW.data->>'project_id' IS DISTINCT FROM NEW.project_id::text OR NEW.data->>'package_id' IS DISTINCT FROM NEW.package_id::text OR NEW.data->>'version' IS DISTINCT FROM NEW.version::text OR NEW.data->>'revision' IS DISTINCT FROM NEW.revision::text OR NEW.data->>'is_active' IS DISTINCT FROM NEW.is_active::text OR NEW.data->'source_snapshot' IS DISTINCT FROM NEW.source_snapshot OR (NEW.data->>'created_at')::timestamptz IS DISTINCT FROM NEW.created_at OR (NEW.data->>'updated_at')::timestamptz IS DISTINCT FROM NEW.updated_at OR nullif(NEW.data->>'created_by_id','')::uuid IS DISTINCT FROM NEW.created_by_id OR nullif(NEW.data->>'updated_by_id','')::uuid IS DISTINCT FROM NEW.updated_by_id THEN RAISE EXCEPTION 'Quality case projection mismatch';END IF;
 IF jsonb_typeof(NEW.data->'certificates') IS DISTINCT FROM 'array' OR jsonb_array_length(NEW.data->'certificates')<>5 OR (SELECT count(DISTINCT x->>'code') FROM jsonb_array_elements(NEW.data->'certificates') x WHERE x->>'code' IN('TC','TR','MTC','RTC','Warranty'))<>5 THEN RAISE EXCEPTION 'Retain all five required certificates';END IF;
 IF NEW.updated_by_id IS NULL OR NEW.created_by_id IS NULL THEN RAISE EXCEPTION 'Quality requires authenticated attribution';END IF;
 IF TG_OP='INSERT' AND (NEW.version<>1 OR NEW.revision<>1) THEN RAISE EXCEPTION 'Quality begins at version and revision one';END IF;
 IF TG_OP='UPDATE' THEN
  IF NEW.id<>OLD.id OR NEW.project_id<>OLD.project_id OR NEW.package_id<>OLD.package_id OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'Quality ownership and audit must be retained';END IF;
  IF NEW.revision=OLD.revision THEN
   IF NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot THEN RAISE EXCEPTION 'Obsolete quality source requires a revision';END IF;
  ELSIF NEW.revision<>OLD.revision+1 OR NEW.source_snapshot=OLD.source_snapshot OR EXISTS(SELECT 1 FROM jsonb_array_elements(NEW.data->'certificates') x WHERE (x->>'verified')::boolean OR x->'verified_at'<>'null'::jsonb OR x->>'verified_by_id'<>'') THEN RAISE EXCEPTION 'Quality revision resets all attestations and retains history';END IF;
 END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.protect_purchase_quality_certificate() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NEW.title IS DISTINCT FROM (CASE NEW.code WHEN 'TC' THEN 'Type Test Certificate' WHEN 'TR' THEN 'Routine Test Report' WHEN 'MTC' THEN 'Material Test Certificate' WHEN 'RTC' THEN 'Routine / Galvanizing Certificate' WHEN 'Warranty' THEN 'Warranty / Guarantee Undertaking' END) THEN RAISE EXCEPTION 'Required certificate identity is fixed';END IF;
 IF NEW.data->>'code' IS DISTINCT FROM NEW.code OR NEW.data->>'title' IS DISTINCT FROM NEW.title OR NEW.data->>'cert_number' IS DISTINCT FROM NEW.cert_number OR NEW.data->>'issued_by' IS DISTINCT FROM NEW.issued_by OR nullif(NEW.data->>'valid_until','')::date IS DISTINCT FROM NEW.valid_until OR NEW.data->>'file_name' IS DISTINCT FROM NEW.file_name OR (NEW.data->>'verified')::boolean IS DISTINCT FROM NEW.verified OR (NEW.data->>'verified_at')::timestamptz IS DISTINCT FROM NEW.verified_at OR nullif(NEW.data->>'verified_by_id','')::uuid IS DISTINCT FROM NEW.verified_by_id OR (NEW.data->>'is_active')::boolean IS DISTINCT FROM NEW.is_active OR (NEW.data->>'created_at')::timestamptz IS DISTINCT FROM NEW.created_at OR (NEW.data->>'updated_at')::timestamptz IS DISTINCT FROM NEW.updated_at OR nullif(NEW.data->>'created_by_id','')::uuid IS DISTINCT FROM NEW.created_by_id OR nullif(NEW.data->>'updated_by_id','')::uuid IS DISTINCT FROM NEW.updated_by_id THEN RAISE EXCEPTION 'Certificate projection mismatch';END IF;
 IF NEW.created_by_id IS NULL OR NEW.updated_by_id IS NULL OR NEW.verified AND (NEW.verified_at<NEW.created_at OR NEW.verified_at>NEW.updated_at OR NEW.valid_until IS NOT NULL AND NEW.valid_until<(NEW.verified_at AT TIME ZONE 'Asia/Kolkata')::date) THEN RAISE EXCEPTION 'Use actual dated quality attribution';END IF;
 IF TG_OP='UPDATE' THEN
  IF NEW.case_id<>OLD.case_id OR NEW.code<>OLD.code OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id THEN RAISE EXCEPTION 'Certificate ownership must be retained';END IF;
  IF (NEW.cert_number,NEW.issued_by,NEW.valid_until,NEW.file_name) IS DISTINCT FROM (OLD.cert_number,OLD.issued_by,OLD.valid_until,OLD.file_name) AND NEW.verified THEN RAISE EXCEPTION 'Changed certificate details require renewed verification';END IF;
 END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.protect_purchase_quality_history() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Quality evidence and history cannot be erased';END $$;

CREATE FUNCTION public.protect_purchase_site_receipts() RETURNS trigger
    LANGUAGE plpgsql
    AS $_$ DECLARE x purchase_dispatches;c purchase_client_clearances;q purchase_quality_cases;r purchase_inspection_reports;i purchase_inspections;d jsonb;expected jsonb;BEGIN
 SELECT * INTO x FROM purchase_dispatches WHERE id=NEW.dispatch_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=NEW.dispatch_version AND status='dispatched' AND is_active;
 IF x.id IS NULL THEN RAISE EXCEPTION 'Site receipt needs its current dispatched material';END IF;
 SELECT * INTO c FROM purchase_client_clearances WHERE id=x.clearance_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=x.clearance_version AND status='cleared' AND is_active;
 IF c.id IS NULL THEN RAISE EXCEPTION 'Site receipt needs its current cleared client MDCC';END IF;
 SELECT * INTO q FROM purchase_quality_cases WHERE id=c.quality_id AND version=c.quality_version AND is_active;
 IF q.id IS NULL OR (SELECT count(*) FROM purchase_quality_certificates WHERE case_id=q.id)<>5 OR EXISTS(SELECT 1 FROM purchase_quality_certificates WHERE case_id=q.id AND (NOT is_active OR NOT verified OR valid_until IS NOT NULL AND valid_until<(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date)) THEN RAISE EXCEPTION 'Site receipt requires current verified quality';END IF;
 SELECT * INTO r FROM purchase_inspection_reports WHERE id=q.report_id AND version=q.report_version AND status='issued' AND is_active;
 IF r.id IS NULL THEN RAISE EXCEPTION 'Site receipt needs current issued IRN';END IF;
 SELECT * INTO i FROM purchase_inspections WHERE id=r.inspection_id AND version=r.inspection_version AND status='cleared' AND is_active;
 IF i.id IS NULL OR NOT EXISTS(SELECT 1 FROM purchase_acceptances WHERE id=i.acceptance_id AND version=i.acceptance_version AND status='accepted' AND is_active) OR NOT EXISTS(SELECT 1 FROM purchase_technical_cases WHERE id=i.technical_id AND version=i.technical_version AND state IN('approved','approved_with_comments') AND is_active) THEN RAISE EXCEPTION 'Site receipt source evidence is no longer current';END IF;
 expected=x.source_snapshot||jsonb_build_object('dispatch_id',x.id,'dispatch_version',x.version,'dispatch_revision',x.revision,'dispatch_date',x.data->'dispatch'->>'dispatch_date','lr_number',x.data->'dispatch'->>'lr_number','transporter_name',x.data->'dispatch'->>'transporter_name');
 IF NEW.source_snapshot IS DISTINCT FROM expected THEN RAISE EXCEPTION 'Site receipt source must equal its current dispatch';END IF;
 IF NEW.data->>'id' IS DISTINCT FROM NEW.id::text OR NEW.data->>'project_id' IS DISTINCT FROM NEW.project_id::text OR NEW.data->>'package_id' IS DISTINCT FROM NEW.package_id::text OR NEW.data->>'version' IS DISTINCT FROM NEW.version::text OR NEW.data->>'revision' IS DISTINCT FROM NEW.revision::text OR NEW.data->>'is_active' IS DISTINCT FROM NEW.is_active::text OR NEW.data->'source_snapshot' IS DISTINCT FROM NEW.source_snapshot OR NEW.data->'receipt'->>'status' IS DISTINCT FROM NEW.status OR (NEW.data->>'created_at')::timestamptz IS DISTINCT FROM NEW.created_at OR (NEW.data->>'updated_at')::timestamptz IS DISTINCT FROM NEW.updated_at OR nullif(NEW.data->>'created_by_id','')::uuid IS DISTINCT FROM NEW.created_by_id OR nullif(NEW.data->>'updated_by_id','')::uuid IS DISTINCT FROM NEW.updated_by_id THEN RAISE EXCEPTION 'Site receipt projection mismatch';END IF;
 IF NEW.created_by_id IS NULL OR NEW.updated_by_id IS NULL THEN RAISE EXCEPTION 'Use authenticated receiving attribution';END IF;
 d=NEW.data->'receipt';
 IF NEW.status='delivered_at_site' AND (length(trim(coalesce(d->>'receiving_officer','')))=0 OR coalesce(d->>'arrival_date','')='') THEN RAISE EXCEPTION 'Delivered receipt needs arrival date and receiving officer';END IF;
 IF coalesce(d->>'packages_condition','') NOT IN('not_checked','intact','minor_damage','quarantine') OR jsonb_typeof(d->'packages_received') IS DISTINCT FROM 'number' OR (d->>'packages_received') !~ '^(0|[1-9][0-9]*)$' OR (d->>'packages_received')::numeric>2147483647 THEN RAISE EXCEPTION 'Use actual nonnegative bounded package count and condition';END IF;
 IF length(coalesce(d->>'receiving_officer',''))>300 OR length(coalesce(d->>'transit_damage_notes',''))>4000 OR length(coalesce(d->>'gate_entry_ref',''))>200 THEN RAISE EXCEPTION 'Site receipt fields exceed limits';END IF;
 IF coalesce(d->>'arrival_date','')<>'' AND ((d->>'arrival_date')::date<(x.data->'dispatch'->>'dispatch_date')::date OR (d->>'arrival_date')::date>(NEW.updated_at AT TIME ZONE 'Asia/Kolkata')::date) THEN RAISE EXCEPTION 'Arrival follows actual dispatch and cannot be future';END IF;
 IF TG_OP='INSERT' AND (NEW.version<>1 OR NEW.revision<>1) THEN RAISE EXCEPTION 'Site receipt begins at version/revision one';END IF;
 IF TG_OP='UPDATE' THEN
 IF NEW.id<>OLD.id OR NEW.project_id<>OLD.project_id OR NEW.package_id<>OLD.package_id OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'Site receipt ownership/audit must be retained';END IF;
 IF NEW.revision=OLD.revision THEN
 IF OLD.status='delivered_at_site' OR NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot THEN RAISE EXCEPTION 'Delivered or obsolete site receipt needs revision';END IF;
 ELSIF NEW.revision<>OLD.revision+1 OR NEW.status<>'pending' OR d IS DISTINCT FROM jsonb_build_object('status','pending','arrival_date','','receiving_officer','','packages_received',0,'packages_condition','not_checked','transit_damage_notes','','gate_entry_ref','') THEN RAISE EXCEPTION 'Site receipt revision resets pending and retains prior evidence';END IF;
 END IF;
 RETURN NEW;END $_$;

CREATE FUNCTION public.protect_purchase_site_receipts_history() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Site receipt and history cannot be erased';END $$;

CREATE FUNCTION public.protect_purchase_sourcing() RETURNS trigger
    LANGUAGE plpgsql
    AS $_$ DECLARE o jsonb;n jsonb;BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_work_packages p WHERE p.id=NEW.package_id AND p.project_id=NEW.project_id) THEN RAISE EXCEPTION 'Sourcing package belongs to another project';END IF;
 IF NEW.data->>'id' IS DISTINCT FROM NEW.id::text OR NEW.data->>'project_id' IS DISTINCT FROM NEW.project_id::text OR NEW.data->>'package_id' IS DISTINCT FROM NEW.package_id::text OR NEW.data->>'approver_id' IS DISTINCT FROM NEW.approver_id::text OR NEW.data->>'state' IS DISTINCT FROM NEW.state OR NEW.data->>'version' IS DISTINCT FROM NEW.version::text OR NEW.data->'source_snapshot' IS DISTINCT FROM NEW.source_snapshot OR NEW.data->>'is_active' IS DISTINCT FROM NEW.is_active::text THEN RAISE EXCEPTION 'Sourcing record projection does not match owned state';END IF;
 IF jsonb_typeof(NEW.data->'offers') IS DISTINCT FROM 'array' OR jsonb_array_length(NEW.data->'offers')>50 THEN RAISE EXCEPTION 'Sourcing offers must be a bounded array';END IF;
 FOR o IN SELECT value FROM jsonb_array_elements(NEW.data->'offers') LOOP
  IF jsonb_typeof(o) IS DISTINCT FROM 'object' OR coalesce(o->>'id','') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$' OR coalesce(o->>'state','') NOT IN('draft','sent','no_bid','quoted') OR o->>'is_active' IS DISTINCT FROM 'true' OR NOT EXISTS(SELECT 1 FROM suppliers WHERE id::text=o->>'supplier_id') THEN RAISE EXCEPTION 'Sourcing offers require owned identity, supplier and state';END IF;
 END LOOP;
 IF EXISTS(SELECT 1 FROM jsonb_array_elements(NEW.data->'offers') GROUP BY value->>'id' HAVING count(*)>1) OR EXISTS(SELECT 1 FROM jsonb_array_elements(NEW.data->'offers') GROUP BY value->>'supplier_id' HAVING count(*)>1) THEN RAISE EXCEPTION 'Sourcing offers must have unique identities and suppliers';END IF;
 IF NEW.state IN('pending','approved','released') AND (jsonb_typeof(NEW.data->'decision_snapshot') IS DISTINCT FROM 'object' OR NEW.data->>'selected_offer_id' IS NULL OR NEW.data->>'selected_offer_id'='' OR NEW.data->>'proposed_by_id' IS NULL OR NEW.data->>'proposed_by_id'='' OR NEW.data->>'proposed_by_id'=NEW.approver_id::text) THEN RAISE EXCEPTION 'Independent sourcing selection evidence is required';END IF;
 IF NEW.state IN('pending','approved','released') AND (NEW.data->'decision_snapshot'->'source' IS DISTINCT FROM NEW.source_snapshot OR NEW.data->'decision_snapshot'->'terms' IS DISTINCT FROM NEW.data->'loi_terms' OR coalesce(NEW.data->'decision_snapshot'->>'financial_reference','')='' OR NOT EXISTS(SELECT 1 FROM jsonb_array_elements(NEW.data->'offers') WHERE value->>'id'=NEW.data->>'selected_offer_id' AND value=NEW.data->'decision_snapshot'->'offer' AND value->>'state'='quoted' AND value->>'compliant'='true')) THEN RAISE EXCEPTION 'Selection must capture its exact source, offer, terms and Finance evidence';END IF;
 IF NEW.state IN('approved','released') AND (NEW.data->>'approved_by_id' IS DISTINCT FROM NEW.approver_id::text OR NEW.data->>'approved_at' IS NULL OR NEW.data->>'approved_by_id'=NEW.data->>'proposed_by_id') THEN RAISE EXCEPTION 'Independent sourcing approval is required';END IF;
 IF NEW.state='released' AND (coalesce(NEW.data->>'loi_reference','')='' OR NEW.data->>'released_at' IS NULL OR coalesce(NEW.data->>'loi_dispatch_reference','')='') THEN RAISE EXCEPTION 'Durable LOI and dispatch evidence are required';END IF;
 IF TG_OP='INSERT' AND NEW.state<>'draft' THEN RAISE EXCEPTION 'Sourcing must begin in draft';END IF;
 IF TG_OP='UPDATE' THEN
  IF NOT ((OLD.state='draft' AND (NEW.state IN('draft','pending') OR (NEW.state='approved' AND coalesce(NEW.data->>'award_reference','')<>''))) OR (OLD.state='pending' AND NEW.state IN('draft','approved')) OR (OLD.state='approved' AND NEW.state IN('draft','released'))) THEN RAISE EXCEPTION 'Invalid sourcing transition';END IF;
  IF OLD.state='released' THEN RAISE EXCEPTION 'Released LOI is immutable';END IF;
  IF NEW.id IS DISTINCT FROM OLD.id OR NEW.project_id IS DISTINCT FROM OLD.project_id OR NEW.package_id IS DISTINCT FROM OLD.package_id OR NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot OR NEW.created_at IS DISTINCT FROM OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'Sourcing ownership/source/version is immutable';END IF;
  IF OLD.state IN('pending','approved') AND NEW.state<>'draft' AND (NEW.data->'decision_snapshot' IS DISTINCT FROM OLD.data->'decision_snapshot' OR NEW.data->'offers' IS DISTINCT FROM OLD.data->'offers' OR NEW.approver_id IS DISTINCT FROM OLD.approver_id) THEN RAISE EXCEPTION 'Submitted sourcing evidence is immutable';END IF;
  FOR o IN SELECT value FROM jsonb_array_elements(OLD.data->'offers') LOOP
   SELECT value INTO n FROM jsonb_array_elements(NEW.data->'offers') WHERE value->>'id'=o->>'id';
   IF NOT FOUND OR n->>'supplier_id' IS DISTINCT FROM o->>'supplier_id' OR n->>'created_at' IS DISTINCT FROM o->>'created_at' OR n->>'created_by_id' IS DISTINCT FROM o->>'created_by_id' THEN RAISE EXCEPTION 'Sourcing offer ownership must be retained';END IF;
  END LOOP;
 END IF;
 IF coalesce(NEW.data->>'award_reference','')<>'' AND NEW.state IN('approved','released') THEN
 IF NOT EXISTS(SELECT 1 FROM quotation_approval_sets q, LATERAL jsonb_array_elements(q.attempts) a WHERE q.project_id=NEW.project_id AND q.is_active AND a->>'id'=NEW.data->>'award_reference' AND a->>'state'='approved' AND a->>'reviewed_by_id'=NEW.data->>'approved_by_id' AND a->>'submitted_by_id'=NEW.data->>'proposed_by_id' AND a->>'vendor_id'=NEW.data->'decision_snapshot'->'offer'->>'supplier_id' AND a->>'quote_key'=NEW.data->'decision_snapshot'->'offer'->>'evidence_reference') THEN RAISE EXCEPTION 'Award must carry existing independent Finance evidence';END IF;
 END IF;
 RETURN NEW;END $_$;

CREATE FUNCTION public.protect_purchase_sourcing_allocation() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE q numeric;a numeric;BEGIN
 PERFORM pg_advisory_xact_lock(hashtextextended(NEW.project_id::text||':'||NEW.demand_id::text,0));
 IF NOT EXISTS(SELECT 1 FROM purchase_sourcing_cases WHERE id=NEW.case_id AND project_id=NEW.project_id AND source_snapshot->>'demand_id'=NEW.demand_id::text) THEN RAISE EXCEPTION 'Allocation must belong to its exact sourcing demand';END IF;
 IF TG_OP='UPDATE' AND (OLD.state='released' OR NEW.case_id<>OLD.case_id OR NEW.project_id<>OLD.project_id OR NEW.demand_id<>OLD.demand_id OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.case_version<=OLD.case_version) THEN RAISE EXCEPTION 'Allocation identity, committed LOI and audit ownership must be retained';END IF;
 IF TG_OP='UPDATE' AND OLD.is_active AND NEW.is_active AND (NEW.financial_reference<>OLD.financial_reference OR NEW.currency<>OLD.currency OR NEW.quantity<>OLD.quantity OR NEW.amount<>OLD.amount OR NEW.approved_quantity<>OLD.approved_quantity OR NEW.approved_amount<>OLD.approved_amount) THEN RAISE EXCEPTION 'Submitted allocation evidence is immutable';END IF;
 IF NEW.is_active THEN
  SELECT coalesce(sum(quantity),0),coalesce(sum(amount) FILTER(WHERE financial_reference=NEW.financial_reference AND currency=NEW.currency),0) INTO q,a FROM purchase_sourcing_allocations WHERE project_id=NEW.project_id AND demand_id=NEW.demand_id AND is_active AND case_id<>NEW.case_id;
  IF q+NEW.quantity>NEW.approved_quantity OR a+NEW.amount>NEW.approved_amount THEN RAISE EXCEPTION 'Approved purchasing capacity already reserved';END IF;
 END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.protect_purchase_sourcing_history() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Sourcing records and history must be retained';END $$;

CREATE FUNCTION public.protect_purchase_technical() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_sourcing_cases s WHERE s.id=NEW.source_case_id AND s.project_id=NEW.project_id AND s.package_id=NEW.package_id AND s.state='released' AND s.is_active AND NEW.source_snapshot->>'case_id'=s.id::text AND NEW.source_snapshot->>'version'=s.version::text AND NEW.source_snapshot->>'loi_reference'=s.data->>'loi_reference') THEN RAISE EXCEPTION 'Technical kit requires its exact released project LOI';END IF;
 IF NEW.data->>'id' IS DISTINCT FROM NEW.id::text OR NEW.data->>'project_id' IS DISTINCT FROM NEW.project_id::text OR NEW.data->>'package_id' IS DISTINCT FROM NEW.package_id::text OR NEW.data->>'version' IS DISTINCT FROM NEW.version::text OR NEW.data->>'revision' IS DISTINCT FROM NEW.revision::text OR NEW.data->>'state' IS DISTINCT FROM NEW.state OR NEW.data->>'is_active' IS DISTINCT FROM NEW.is_active::text OR NEW.data->'source_snapshot' IS DISTINCT FROM NEW.source_snapshot THEN RAISE EXCEPTION 'Technical state projection mismatch';END IF;
 IF jsonb_typeof(NEW.data->'documents') IS DISTINCT FROM 'array' OR jsonb_array_length(NEW.data->'documents')<>6 THEN RAISE EXCEPTION 'Technical kit requires its six owned checklist records';END IF;
 IF TG_OP='INSERT' AND (NEW.version<>1 OR NEW.revision<>1 OR NEW.state<>'draft') THEN RAISE EXCEPTION 'Technical kits begin in pending draft';END IF;
 IF TG_OP='UPDATE' AND (NEW.id<>OLD.id OR NEW.project_id<>OLD.project_id OR NEW.package_id<>OLD.package_id OR NEW.source_case_id<>OLD.source_case_id OR NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1 OR NEW.revision<OLD.revision OR NEW.revision>OLD.revision+1) THEN RAISE EXCEPTION 'Technical ownership, revision and audit history must be retained';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.protect_purchase_technical_history() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Technical evidence and history are immutable';END $$;

CREATE FUNCTION public.protect_purchase_work_package() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE step jsonb;prior jsonb;i integer:=0;
BEGIN
 IF TG_OP='UPDATE' AND(NEW.id<>OLD.id OR NEW.project_id<>OLD.project_id OR NEW.indent_id<>OLD.indent_id OR NEW.demand_id<>OLD.demand_id OR NEW.code<>OLD.code OR NEW.description<>OLD.description OR NEW.source_snapshot<>OLD.source_snapshot OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1)THEN RAISE EXCEPTION 'Package identity/source/version is immutable';END IF;
 IF NOT EXISTS(SELECT 1 FROM purchase_indents WHERE id=NEW.indent_id AND project_id=NEW.project_id) OR NOT EXISTS(SELECT 1 FROM project_material_requirements WHERE id=NEW.demand_id AND project_id=NEW.project_id) OR NOT EXISTS(SELECT 1 FROM purchase_indent_releases WHERE indent_id=NEW.indent_id AND demand_id=NEW.demand_id) THEN RAISE EXCEPTION 'Package must reference its released project indent line';END IF;
 FOR step IN SELECT value FROM jsonb_array_elements(NEW.steps)LOOP
 i:=i+1;
 IF (step->>'number')::integer IS DISTINCT FROM i OR step->>'state' IS NULL OR(i<=2 AND step->>'state'<>'source_linked')OR(i>2 AND step->>'state' NOT IN('not_started','preparing','blocked'))THEN RAISE EXCEPTION 'Invalid purchase preparation catalogue/state';END IF;
 IF TG_OP='UPDATE'THEN prior:=OLD.steps->(i-1);IF step->'number' IS DISTINCT FROM prior->'number' OR step->'title' IS DISTINCT FROM prior->'title' OR step->'phase' IS DISTINCT FROM prior->'phase' OR step->'evidence_required' IS DISTINCT FROM prior->'evidence_required'THEN RAISE EXCEPTION 'Frozen step definition is immutable';END IF;END IF;
 END LOOP;RETURN NEW;
END $$;

CREATE FUNCTION public.protect_quotation_approval_history() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Quotation approval records/history must be retained';END $$;

CREATE FUNCTION public.protect_quotation_approval_scope() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE a jsonb;n jsonb;l jsonb;nl jsonb;src jsonb;k text;review_fields text[];line_fields text[];baseline jsonb;allocation jsonb;bind_budget boolean;BEGIN
 IF TG_OP='UPDATE' AND (NEW.bo_id IS DISTINCT FROM OLD.bo_id OR NEW.project_id IS DISTINCT FROM OLD.project_id OR NEW.created_at IS DISTINCT FROM OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1) THEN RAISE EXCEPTION 'Quotation approval ownership/version is immutable';END IF;
 IF TG_OP='UPDATE' THEN
  FOR a IN SELECT value FROM jsonb_array_elements(OLD.attempts) LOOP
   SELECT value INTO n FROM jsonb_array_elements(NEW.attempts) WHERE value->>'id'=a->>'id';
   IF NOT FOUND THEN RAISE EXCEPTION 'Quotation submissions must be retained';END IF;
   IF a->>'state'<>'pending_accounts' THEN
    IF n IS DISTINCT FROM a THEN RAISE EXCEPTION 'Decided quotations are immutable';END IF;
    CONTINUE;
   END IF;
   review_fields:=ARRAY['state','lines','reviewed_by_id','reviewed_at','decision_id','return_action','review_comment'];
   line_fields:=ARRAY['decision','comment'];
   bind_budget:=a->>'state'='pending_accounts' AND n->>'state'='approved';
   IF bind_budget THEN
    review_fields:=review_fields||ARRAY['budget_revision','budget_issue'];
    line_fields:=line_fields||ARRAY['budget','difference'];
    SELECT approved_snapshot INTO baseline FROM project_budgets WHERE project_id=NEW.project_id AND is_active;
    IF baseline IS NULL OR baseline='null'::jsonb OR coalesce((n->>'budget_revision')::integer,0)<1 OR n->>'budget_revision' IS DISTINCT FROM baseline->>'revision' OR n->>'currency' IS DISTINCT FROM baseline->'data'->>'currency' OR coalesce(n->>'budget_issue','')<>'' THEN RAISE EXCEPTION 'Vendor approval requires the current approved project budget';END IF;
   END IF;
   IF jsonb_array_length(coalesce(a->'proposals','[]'))=0 THEN
    IF n-review_fields IS DISTINCT FROM a-review_fields THEN RAISE EXCEPTION 'Submitted quotation source is immutable';END IF;
    src:=a;
   ELSE
    IF n-(review_fields||ARRAY['quote_key','document_id','revision','vendor_id','vendor','reference','total','verification_fingerprint','terms','valid_until']) IS DISTINCT FROM a-(review_fields||ARRAY['quote_key','document_id','revision','vendor_id','vendor','reference','total','verification_fingerprint','terms','valid_until']) THEN RAISE EXCEPTION 'Submitted shortlist and executive reasons are immutable';END IF;
    SELECT value->'snapshot' INTO src FROM jsonb_array_elements(a->'proposals') WHERE value->>'quote_key'=n->>'quote_key';
    IF NOT FOUND THEN RAISE EXCEPTION 'Finance must choose a submitted proposal';END IF;
    FOREACH k IN ARRAY ARRAY['quote_key','document_id','revision','vendor_id','vendor','reference','total','verification_fingerprint','terms','valid_until'] LOOP
     IF n->k IS DISTINCT FROM src->k THEN RAISE EXCEPTION 'Selected proposal source must match the frozen snapshot';END IF;
    END LOOP;
    IF n->>'state'='pending_accounts' AND n IS DISTINCT FROM a THEN RAISE EXCEPTION 'Waiting shortlist is frozen';END IF;
   END IF;
   IF n->>'state'='rejected' AND jsonb_array_length(coalesce(a->'proposals','[]'))>0 AND coalesce(n->>'return_action','') NOT IN('negotiate','change_vendor') THEN RAISE EXCEPTION 'Rejected shortlist requires recovery instruction';END IF;
   IF n->>'state'='approved' AND coalesce(n->>'return_action','')<>'' THEN RAISE EXCEPTION 'Approved shortlist cannot be returned';END IF;
   IF jsonb_array_length(n->'lines')<>jsonb_array_length(src->'lines') THEN RAISE EXCEPTION 'Every submitted item must be retained';END IF;
   FOR l IN SELECT value FROM jsonb_array_elements(src->'lines') LOOP
    SELECT value INTO nl FROM jsonb_array_elements(n->'lines') WHERE value->>'line_id'=l->>'line_id';
    IF NOT FOUND OR nl-line_fields IS DISTINCT FROM l-line_fields THEN RAISE EXCEPTION 'Submitted item source is immutable';END IF;
    IF bind_budget THEN
     SELECT value INTO allocation FROM jsonb_array_elements(baseline->'data'->'allocations') WHERE value->>'bo_id'=NEW.bo_id::text AND value->>'line_id'=nl->>'line_id' AND coalesce((value->>'is_active')::boolean,false);
     IF NOT FOUND THEN
      IF nl->>'budget' IS NOT NULL OR nl->>'difference' IS NOT NULL THEN RAISE EXCEPTION 'Unallocated items must retain unavailable comparisons';END IF;
     ELSIF nl->>'budget' IS NULL OR nl->>'difference' IS NULL OR (nl->>'budget')::numeric IS DISTINCT FROM (allocation->>'amount')::numeric OR (nl->>'difference')::numeric IS DISTINCT FROM (nl->>'quoted')::numeric-(allocation->>'amount')::numeric THEN RAISE EXCEPTION 'Finance item comparison must match its approved allocation';END IF;
    END IF;
   END LOOP;
  END LOOP;
 END IF;
 IF NOT EXISTS(SELECT 1 FROM purchase_bo_lists WHERE id=NEW.bo_id AND project_id=NEW.project_id) THEN RAISE EXCEPTION 'Quotation approval BO belongs to another project';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION public.protect_quotation_evidence() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Quotation evidence and revisions are immutable';END $$;

CREATE FUNCTION public.protect_quotation_owner() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF TG_OP='UPDATE' AND ROW(NEW.bo_id,NEW.project_id,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.bo_id,OLD.project_id,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Quotation BO ownership is immutable';END IF;
 IF NOT EXISTS(SELECT 1 FROM purchase_bo_lists WHERE id=NEW.bo_id AND project_id=NEW.project_id) THEN RAISE EXCEPTION 'Quotation BO belongs to another project';END IF;RETURN NEW;END $$;

CREATE FUNCTION public.protect_quotation_revision_scope() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_indent_documents WHERE id=NEW.document_id AND bo_id=NEW.bo_id) THEN RAISE EXCEPTION 'Quotation document belongs to another BO';END IF;RETURN NEW;END $$;

CREATE FUNCTION public.protect_reading_evidence() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Document originals, results and history are immutable'; END $$;

CREATE FUNCTION public.protect_reading_source() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Document originals cannot be deleted'; END IF;
IF ROW(NEW.module,NEW.entity_id,NEW.filename,NEW.format,NEW.size_bytes,NEW.sha256,NEW.original,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.module,OLD.entity_id,OLD.filename,OLD.format,OLD.size_bytes,OLD.sha256,OLD.original,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Document original is immutable'; END IF;
IF OLD.status='ready' THEN RAISE EXCEPTION 'Completed reading is immutable'; END IF; RETURN NEW; END $$;

CREATE FUNCTION public.protect_template_revision() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Published template revisions are immutable; publish a new revision'; END; $$;

CREATE FUNCTION public.purchase_fms_history_immutable() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN RAISE EXCEPTION 'Purchase ownership history is immutable'; END; $$;

CREATE FUNCTION public.refuse_purchase_work_evidence_mutation() RETURNS trigger
    LANGUAGE plpgsql
    AS $$BEGIN RAISE EXCEPTION 'Purchase package evidence cannot be removed or rewritten';END $$;

CREATE FUNCTION public.reject_calendar_history_change() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Calendar history is immutable'; END $$;

CREATE FUNCTION public.reject_project_import_change() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Project import provenance is immutable'; END $$;

CREATE FUNCTION public.reject_project_metadata_change() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'Project metadata history is immutable'; END $$;

CREATE FUNCTION tender.protect_cloud_reservation() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF TG_OP<>'UPDATE' THEN RAISE EXCEPTION 'Cloud reservations cannot be deleted'; END IF;
 IF OLD.state<>'pending' OR ROW(NEW.id,NEW.draft_id,NEW.job_id,NEW.actor_id,NEW.request_key,NEW.fingerprint,NEW.provider,NEW.model,NEW.reserved_paise,NEW.created_at)
 IS DISTINCT FROM ROW(OLD.id,OLD.draft_id,OLD.job_id,OLD.actor_id,OLD.request_key,OLD.fingerprint,OLD.provider,OLD.model,OLD.reserved_paise,OLD.created_at)
 THEN RAISE EXCEPTION 'Cloud reservation evidence is immutable'; END IF;
 RETURN NEW;
END $$;

CREATE FUNCTION tender.protect_tsc_format_snapshot() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF ROW(NEW.format_id,NEW.format_version) IS DISTINCT FROM ROW(OLD.format_id,OLD.format_version) THEN RAISE EXCEPTION 'TSC format snapshot is immutable'; END IF; RETURN NEW;
END $$;

CREATE FUNCTION tender.protect_tsc_inputs() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF TG_OP<>'UPDATE' THEN RAISE EXCEPTION 'TSC snapshots require retention-aware cleanup'; END IF;
 IF ROW(NEW.id,NEW.draft_id,NEW.draft_version,NEW.actor_id,NEW.operation_key,NEW.mode,NEW.model,NEW.input_cipher,NEW.input_hash,NEW.created_at)
 IS DISTINCT FROM ROW(OLD.id,OLD.draft_id,OLD.draft_version,OLD.actor_id,OLD.operation_key,OLD.mode,OLD.model,OLD.input_cipher,OLD.input_hash,OLD.created_at)
 THEN RAISE EXCEPTION 'TSC input snapshot is immutable'; END IF;
 RETURN NEW;
END $$;

CREATE FUNCTION tender.protect_tsc_model_call() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF TG_OP<>'UPDATE' OR OLD.state IN('completed','failed','uncertain') THEN RAISE EXCEPTION 'Model call evidence is immutable';END IF;
 IF ROW(NEW.id,NEW.job_id,NEW.draft_id,NEW.actor_id,NEW.task_id,NEW.request_key,NEW.provider,NEW.model,NEW.purpose,NEW.manifest,NEW.pricing,NEW.reserved_paise,NEW.created_at)
 IS DISTINCT FROM ROW(OLD.id,OLD.job_id,OLD.draft_id,OLD.actor_id,OLD.task_id,OLD.request_key,OLD.provider,OLD.model,OLD.purpose,OLD.manifest,OLD.pricing,OLD.reserved_paise,OLD.created_at)
 THEN RAISE EXCEPTION 'Model request identity is immutable';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION tender.protect_tsc_workspace() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF TG_OP<>'UPDATE' THEN RAISE EXCEPTION 'TSC workspace requires retention-aware cleanup';END IF;
 IF ROW(NEW.id,NEW.draft_id,NEW.draft_version,NEW.actor_id,NEW.operation_key,NEW.request_hash,NEW.mode,NEW.snapshot_cipher,NEW.snapshot_hash,NEW.created_at)
 IS DISTINCT FROM ROW(OLD.id,OLD.draft_id,OLD.draft_version,OLD.actor_id,OLD.operation_key,OLD.request_hash,OLD.mode,OLD.snapshot_cipher,OLD.snapshot_hash,OLD.created_at)
 THEN RAISE EXCEPTION 'TSC source snapshot is immutable';END IF;
 RETURN NEW;END $$;

CREATE FUNCTION tender.protect_tsc_workspace_task() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
 IF TG_OP<>'UPDATE' OR OLD.state IN('ready','failed') THEN RAISE EXCEPTION 'TSC task evidence is immutable';END IF;
 IF ROW(NEW.id,NEW.job_id,NEW.draft_id,NEW.actor_id,NEW.operation_key,NEW.request_hash,NEW.question_id,NEW.provider,NEW.stronger,NEW.created_at)
 IS DISTINCT FROM ROW(OLD.id,OLD.job_id,OLD.draft_id,OLD.actor_id,OLD.operation_key,OLD.request_hash,OLD.question_id,OLD.provider,OLD.stronger,OLD.created_at)
 THEN RAISE EXCEPTION 'TSC task identity is immutable';END IF;RETURN NEW;END $$;

CREATE FUNCTION tender.tsc_question_key(value text) RETURNS text
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
    AS $$
 SELECT lower(regexp_replace(btrim(value), '[[:space:][:punct:]]+', '', 'g'))
$$;

SET LOCAL default_table_access_method = heap;

CREATE TABLE auth.access_sessions (
    token_hash text NOT NULL,
    user_id uuid NOT NULL,
    family_id uuid NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    revoked_at timestamp with time zone
);

CREATE TABLE auth.administration_changes (
    id bigint NOT NULL,
    entity_type text NOT NULL,
    entity_id uuid NOT NULL,
    action text NOT NULL,
    actor_id uuid,
    before_snapshot jsonb,
    after_snapshot jsonb,
    occurred_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);

ALTER TABLE auth.administration_changes ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME auth.administration_changes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

CREATE TABLE auth.auth_bootstrap_identity (
    singleton boolean DEFAULT true NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT auth_bootstrap_identity_singleton_check CHECK (singleton)
);

CREATE TABLE auth.auth_credential_changes (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    actor_id uuid NOT NULL,
    action text NOT NULL,
    reason text NOT NULL,
    credential_version bigint NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT auth_credential_changes_reason_check CHECK (((length(reason) >= 10) AND (length(reason) <= 1000)))
);

CREATE TABLE auth.auth_credential_emails (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    credential_version bigint NOT NULL,
    payload bytea,
    payload_hash text NOT NULL,
    state text DEFAULT 'pending'::text NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    lease_id uuid,
    lease_until timestamp with time zone,
    next_attempt_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT auth_credential_emails_state_check CHECK ((state = ANY (ARRAY['pending'::text, 'sending'::text, 'delivered'::text, 'failed'::text, 'cancelled'::text])))
);

CREATE TABLE auth.auth_departments (
    id uuid NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    manager_id uuid,
    is_active boolean DEFAULT true NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT auth_departments_code_check CHECK (((length(code) >= 2) AND (length(code) <= 32))),
    CONSTRAINT auth_departments_name_check CHECK (((length(name) >= 2) AND (length(name) <= 150))),
    CONSTRAINT auth_departments_version_check CHECK ((version > 0))
);

CREATE TABLE auth.auth_offboarding_cases (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    manager_id uuid,
    profile_version bigint NOT NULL,
    state text NOT NULL,
    reason text NOT NULL,
    items jsonb NOT NULL,
    approval_by_id uuid,
    approved_at timestamp with time zone,
    is_active boolean DEFAULT true NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT auth_offboarding_cases_check CHECK ((user_id <> manager_id)),
    CONSTRAINT auth_offboarding_cases_check1 CHECK (((state <> 'approved'::text) OR ((approval_by_id IS NOT NULL) AND (approval_by_id = manager_id) AND (approved_at IS NOT NULL)))),
    CONSTRAINT auth_offboarding_cases_items_check CHECK ((jsonb_typeof(items) = 'array'::text)),
    CONSTRAINT auth_offboarding_cases_reason_check CHECK (((length(reason) >= 10) AND (length(reason) <= 1000))),
    CONSTRAINT auth_offboarding_cases_state_check CHECK ((state = ANY (ARRAY['pending'::text, 'planning'::text, 'approved'::text, 'completed'::text, 'cancelled'::text]))),
    CONSTRAINT auth_offboarding_cases_version_check CHECK ((version > 0))
);

CREATE TABLE auth.auth_offboarding_changes (
    id uuid NOT NULL,
    case_id uuid NOT NULL,
    version bigint NOT NULL,
    action text NOT NULL,
    reason text NOT NULL,
    snapshot jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid NOT NULL,
    CONSTRAINT auth_offboarding_changes_reason_check CHECK (((length(reason) >= 10) AND (length(reason) <= 1000)))
);

CREATE TABLE auth.auth_organisation_changes (
    id uuid NOT NULL,
    entity_id uuid NOT NULL,
    entity_type text NOT NULL,
    version bigint NOT NULL,
    reason text NOT NULL,
    snapshot jsonb NOT NULL,
    actor_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT auth_organisation_changes_entity_type_check CHECK ((entity_type = ANY (ARRAY['employee'::text, 'department'::text]))),
    CONSTRAINT auth_organisation_changes_reason_check CHECK (((length(reason) >= 10) AND (length(reason) <= 1000)))
);

CREATE TABLE auth.auth_password_recovery (
    token_hash text NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    consumed_at timestamp with time zone,
    CONSTRAINT auth_password_recovery_check CHECK ((expires_at > created_at)),
    CONSTRAINT auth_password_recovery_token_hash_check CHECK ((length(token_hash) = 64))
);

CREATE TABLE auth.auth_personal_contact_changes (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    version bigint NOT NULL,
    reason text NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    actor_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT auth_personal_contact_changes_check CHECK ((actor_id = user_id)),
    CONSTRAINT auth_personal_contact_changes_reason_check CHECK (((length(reason) >= 10) AND (length(reason) <= 1000))),
    CONSTRAINT auth_personal_contact_changes_version_check CHECK ((version > 0))
);

CREATE TABLE auth.auth_reporting_lines (
    user_id uuid NOT NULL,
    department_id uuid NOT NULL,
    manager_id uuid,
    is_company_root boolean DEFAULT false NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT auth_reporting_lines_check CHECK ((user_id IS DISTINCT FROM manager_id)),
    CONSTRAINT auth_reporting_lines_check1 CHECK ((((manager_id IS NOT NULL) AND (NOT is_company_root)) OR ((manager_id IS NULL) AND is_company_root))),
    CONSTRAINT auth_reporting_lines_version_check CHECK ((version > 0))
);

CREATE TABLE auth.auth_user_status_changes (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    actor_id uuid NOT NULL,
    is_active boolean NOT NULL,
    reason text NOT NULL,
    credential_version bigint NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT auth_user_status_changes_reason_check CHECK (((length(reason) >= 10) AND (length(reason) <= 1000)))
);

CREATE TABLE auth.auth_username_changes (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    version bigint NOT NULL,
    previous_username text NOT NULL,
    username text NOT NULL,
    created_by_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT auth_username_changes_check CHECK ((created_by_id = user_id)),
    CONSTRAINT auth_username_changes_check1 CHECK ((previous_username <> username)),
    CONSTRAINT auth_username_changes_version_check CHECK ((version > 0))
);

CREATE TABLE auth.email_changes (
    token_hash text NOT NULL,
    user_id uuid NOT NULL,
    email text NOT NULL,
    previous_email text NOT NULL,
    credential_version bigint NOT NULL,
    created_by_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    consumed_at timestamp with time zone
);

CREATE TABLE auth.login_attempts (
    identifier_hash text NOT NULL,
    failures integer DEFAULT 0 NOT NULL,
    window_started_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    blocked_until timestamp with time zone
);

CREATE TABLE auth.permissions (
    id uuid NOT NULL,
    code character varying(128) NOT NULL,
    description character varying(255) NOT NULL
);

CREATE TABLE auth.refresh_tokens (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    token_hash character varying(64) NOT NULL,
    family_id uuid NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    revoked_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE auth.role_permissions (
    role_id uuid NOT NULL,
    permission_id uuid NOT NULL
);

CREATE TABLE auth.roles (
    id uuid NOT NULL,
    name character varying(64) NOT NULL,
    description character varying(255) NOT NULL,
    is_active boolean NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by_id uuid,
    edit_version bigint DEFAULT 1 NOT NULL
);

CREATE TABLE auth.user_activity (
    user_id uuid NOT NULL,
    current_page character varying(255) NOT NULL,
    last_seen_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE auth.user_profile (
    user_id uuid NOT NULL,
    display_name character varying(100) DEFAULT ''::character varying NOT NULL,
    photo bytea,
    version bigint DEFAULT 1 NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by_id uuid,
    photo_object_key text,
    photo_sha256 text,
    CONSTRAINT user_profile_image_reference CHECK ((((photo_object_key IS NULL) AND (photo_sha256 IS NULL)) OR ((photo_object_key IS NOT NULL) AND ((length(photo_object_key) >= 1) AND (length(photo_object_key) <= 512)) AND (photo_sha256 IS NOT NULL) AND (photo_sha256 ~ '^[0-9a-f]{64}$'::text) AND (photo IS NULL)))),
    CONSTRAINT user_profiles_photo_check CHECK (((photo IS NULL) OR (octet_length(photo) <= 262144))),
    CONSTRAINT user_profiles_version_check CHECK ((version > 0))
);

CREATE TABLE auth.user_roles (
    user_id uuid NOT NULL,
    role_id uuid NOT NULL
);

CREATE TABLE auth.users (
    id uuid NOT NULL,
    username character varying(64) NOT NULL,
    email character varying(255) NOT NULL,
    password_hash character varying(255) NOT NULL,
    is_active boolean NOT NULL,
    is_superuser boolean NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    account_version bigint DEFAULT 0 NOT NULL,
    must_change_password boolean DEFAULT false NOT NULL,
    temporary_password_expires_at timestamp with time zone,
    credential_version bigint DEFAULT 0 NOT NULL,
    first_name text DEFAULT ''::text NOT NULL,
    last_name text DEFAULT ''::text NOT NULL,
    mobile text DEFAULT ''::text NOT NULL,
    edit_version bigint DEFAULT 1 NOT NULL,
    CONSTRAINT users_credential_version_check CHECK ((credential_version >= 0)),
    CONSTRAINT users_first_name_check CHECK ((length(first_name) <= 100)),
    CONSTRAINT users_last_name_check CHECK ((length(last_name) <= 100)),
    CONSTRAINT users_mobile_check CHECK ((length(mobile) <= 24))
);

CREATE TABLE public.activity_events (
    scope_kind text NOT NULL,
    scope_id uuid NOT NULL,
    id uuid NOT NULL,
    module text NOT NULL,
    action text NOT NULL,
    actor_id uuid,
    actor_kind text NOT NULL,
    occurred_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    recorded_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    source_event_id uuid,
    entity_id uuid,
    entity_version bigint,
    reason text,
    CONSTRAINT activity_events_actor_kind_check CHECK ((actor_kind = ANY (ARRAY['user'::text, 'system'::text]))),
    CONSTRAINT activity_events_check CHECK (((actor_kind = 'user'::text) = (actor_id IS NOT NULL))),
    CONSTRAINT activity_events_scope_kind_check CHECK ((scope_kind = ANY (ARRAY['project'::text, 'tender'::text])))
)
PARTITION BY LIST (scope_kind);

CREATE TABLE public.activity_events_project (
    scope_kind text NOT NULL,
    scope_id uuid NOT NULL,
    id uuid NOT NULL,
    module text NOT NULL,
    action text NOT NULL,
    actor_id uuid,
    actor_kind text NOT NULL,
    occurred_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    recorded_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    source_event_id uuid,
    entity_id uuid,
    entity_version bigint,
    reason text,
    CONSTRAINT activity_events_actor_kind_check CHECK ((actor_kind = ANY (ARRAY['user'::text, 'system'::text]))),
    CONSTRAINT activity_events_check CHECK (((actor_kind = 'user'::text) = (actor_id IS NOT NULL))),
    CONSTRAINT activity_events_scope_kind_check CHECK ((scope_kind = ANY (ARRAY['project'::text, 'tender'::text])))
);

CREATE TABLE public.activity_events_tender (
    scope_kind text NOT NULL,
    scope_id uuid NOT NULL,
    id uuid NOT NULL,
    module text NOT NULL,
    action text NOT NULL,
    actor_id uuid,
    actor_kind text NOT NULL,
    occurred_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    recorded_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    source_event_id uuid,
    entity_id uuid,
    entity_version bigint,
    reason text,
    CONSTRAINT activity_events_actor_kind_check CHECK ((actor_kind = ANY (ARRAY['user'::text, 'system'::text]))),
    CONSTRAINT activity_events_check CHECK (((actor_kind = 'user'::text) = (actor_id IS NOT NULL))),
    CONSTRAINT activity_events_scope_kind_check CHECK ((scope_kind = ANY (ARRAY['project'::text, 'tender'::text])))
);

CREATE TABLE public.application_purchase_settings (
    singleton boolean DEFAULT true NOT NULL,
    item_list_mode text DEFAULT 'single'::text NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_by_id uuid,
    CONSTRAINT application_purchase_settings_item_list_mode_check CHECK ((item_list_mode = ANY (ARRAY['single'::text, 'multiple'::text]))),
    CONSTRAINT application_purchase_settings_singleton_check CHECK (singleton)
);

CREATE TABLE public.business_events (
    id uuid NOT NULL,
    source_module character varying(64) NOT NULL,
    entity_type character varying(64) NOT NULL,
    entity_id uuid NOT NULL,
    entity_version bigint NOT NULL,
    delivery_count integer NOT NULL,
    action character varying(64) NOT NULL,
    actor_id uuid,
    actor_kind character varying(16) NOT NULL,
    correlation_id uuid NOT NULL,
    causation_id uuid,
    scope_kind character varying(16) NOT NULL,
    scope_id uuid,
    visibility character varying(16) NOT NULL,
    occurred_at timestamp with time zone NOT NULL,
    recorded_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT business_events_action_check CHECK (((action)::text ~ '^[a-z][a-z0-9_.]{0,63}$'::text)),
    CONSTRAINT business_events_actor_kind_check CHECK (((actor_kind)::text = ANY ((ARRAY['user'::character varying, 'system'::character varying])::text[]))),
    CONSTRAINT business_events_check CHECK (((((actor_kind)::text = 'user'::text) AND (actor_id IS NOT NULL)) OR (((actor_kind)::text = 'system'::text) AND (actor_id IS NULL)))),
    CONSTRAINT business_events_check1 CHECK (((((scope_kind)::text = 'platform'::text) AND (scope_id IS NULL)) OR (((scope_kind)::text = ANY ((ARRAY['project'::character varying, 'tender'::character varying])::text[])) AND (scope_id IS NOT NULL)))),
    CONSTRAINT business_events_delivery_count_check CHECK (((delivery_count >= 0) AND (delivery_count <= 32))),
    CONSTRAINT business_events_entity_type_check CHECK (((entity_type)::text ~ '^[a-z][a-z0-9_.]{0,63}$'::text)),
    CONSTRAINT business_events_entity_version_check CHECK ((entity_version > 0)),
    CONSTRAINT business_events_scope_kind_check CHECK (((scope_kind)::text = ANY ((ARRAY['platform'::character varying, 'project'::character varying, 'tender'::character varying])::text[]))),
    CONSTRAINT business_events_source_module_check CHECK (((source_module)::text ~ '^[a-z][a-z0-9_.]{0,63}$'::text)),
    CONSTRAINT business_events_visibility_check CHECK (((visibility)::text = 'restricted'::text))
);

CREATE TABLE public.catalogue_item_events (
    id uuid NOT NULL,
    subject_id uuid NOT NULL,
    action character varying(40) NOT NULL,
    reason character varying(2000) NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid
);

CREATE TABLE public.catalogue_items (
    id uuid NOT NULL,
    configuration_id uuid NOT NULL,
    manufacturer_id uuid NOT NULL,
    category_id uuid NOT NULL,
    model character varying(120) NOT NULL,
    specifications jsonb NOT NULL,
    snapshot jsonb NOT NULL,
    fingerprint character varying(64) NOT NULL,
    engineering_key character varying(64) NOT NULL,
    description text NOT NULL,
    sku character varying(1000) NOT NULL,
    serial integer,
    status character varying(20) NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    is_active boolean NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT catalogue_items_check CHECK ((is_active = ((status)::text = 'approved'::text))),
    CONSTRAINT catalogue_items_serial_check CHECK (((serial >= 1) AND (serial <= 999))),
    CONSTRAINT catalogue_items_status_check CHECK (((status)::text = ANY ((ARRAY['approved'::character varying, 'retired'::character varying])::text[]))),
    CONSTRAINT catalogue_items_version_check CHECK ((version > 0))
);

CREATE TABLE public.catalogue_units (
    id uuid NOT NULL,
    code character varying(16) NOT NULL,
    name character varying(128) NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by_id uuid,
    legacy_source jsonb,
    source_fingerprint text,
    CONSTRAINT catalogue_units_version_check CHECK ((version > 0))
);

CREATE TABLE public.categories (
    id uuid NOT NULL,
    code character varying(32) NOT NULL,
    name character varying(255) NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by_id uuid,
    legacy_source jsonb,
    source_fingerprint text,
    unit_id uuid NOT NULL,
    CONSTRAINT categories_version_check CHECK ((version > 0))
);

CREATE TABLE public.category_catalogue_changes (
    id uuid NOT NULL,
    kind character varying(16) NOT NULL,
    subject_id uuid NOT NULL,
    action character varying(16) NOT NULL,
    reason character varying(2000) NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    before_snapshot jsonb DEFAULT '{}'::jsonb NOT NULL,
    CONSTRAINT category_catalogue_changes_action_check CHECK (((action)::text = ANY ((ARRAY['create'::character varying, 'update'::character varying])::text[]))),
    CONSTRAINT category_catalogue_changes_kind_check CHECK (((kind)::text = ANY ((ARRAY['category'::character varying, 'subcategory'::character varying, 'unit'::character varying])::text[])))
);

CREATE TABLE public.category_images (
    category_id uuid NOT NULL,
    image bytea NOT NULL,
    updated_by_id uuid NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT category_images_image_check CHECK (((octet_length(image) >= 1) AND (octet_length(image) <= 262144)))
);

CREATE TABLE public.category_subcategories (
    id uuid NOT NULL,
    code character varying(32) NOT NULL,
    name character varying(255) NOT NULL,
    category_id uuid NOT NULL,
    unit_id uuid NOT NULL,
    configuration jsonb NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by_id uuid,
    legacy_source jsonb,
    source_fingerprint text,
    CONSTRAINT category_subcategories_version_check CHECK ((version > 0))
);

CREATE TABLE public.client_contacts (
    id uuid NOT NULL,
    client_id uuid NOT NULL,
    full_name character varying(128) NOT NULL,
    designation character varying(128) NOT NULL,
    department character varying(128) NOT NULL,
    email character varying(255) NOT NULL,
    phone character varying(32) NOT NULL,
    is_primary boolean NOT NULL,
    is_active boolean NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by_id uuid,
    contact_type character varying(16) DEFAULT ''::character varying NOT NULL,
    CONSTRAINT client_contacts_type_check CHECK (((contact_type)::text = ANY ((ARRAY[''::character varying, 'EIC'::character varying, 'HOD'::character varying, 'SITE_CONTACT'::character varying])::text[])))
);

CREATE TABLE public.client_purchase_rule_events (
    id uuid NOT NULL,
    subject_id uuid NOT NULL,
    action character varying(40) NOT NULL,
    reason character varying(2000) NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid
);

CREATE TABLE public.client_purchase_rules (
    id uuid NOT NULL,
    client_id uuid NOT NULL,
    project_id uuid,
    data jsonb NOT NULL,
    is_active boolean NOT NULL,
    version integer NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT client_purchase_rules_version_check CHECK ((version > 0))
);

CREATE TABLE public.client_types (
    id uuid NOT NULL,
    name character varying(128) NOT NULL,
    normalized_name character varying(128) NOT NULL,
    code character varying(32) NOT NULL,
    is_active boolean NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by_id uuid
);

CREATE TABLE public.clients (
    id uuid NOT NULL,
    client_code character varying(20) NOT NULL,
    legal_name character varying(255) NOT NULL,
    normalized_legal_name character varying(255) NOT NULL,
    display_name character varying(255) NOT NULL,
    parent_organization character varying(255) NOT NULL,
    client_type character varying(64) NOT NULL,
    industry character varying(128) NOT NULL,
    gstin character varying(15) NOT NULL,
    pan character varying(10) NOT NULL,
    cin character varying(21) NOT NULL,
    website character varying(255) NOT NULL,
    address_line_1 character varying(255) NOT NULL,
    address_line_2 character varying(255) NOT NULL,
    city character varying(128) NOT NULL,
    state character varying(128) NOT NULL,
    postal_code character varying(24) NOT NULL,
    country character varying(128) NOT NULL,
    legacy_source character varying(128) DEFAULT ''::character varying NOT NULL,
    legacy_source_id character varying(64) DEFAULT ''::character varying NOT NULL,
    legacy_source_row integer,
    legacy_created_at timestamp with time zone,
    legacy_updated_at timestamp with time zone,
    is_active boolean NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by_id uuid
);

CREATE TABLE public.communication_notice_events (
    id uuid NOT NULL,
    notice_id uuid NOT NULL,
    action character varying(30) NOT NULL,
    notice_version integer NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    CONSTRAINT communication_notice_events_action_check CHECK (((action)::text = ANY ((ARRAY['delivered'::character varying, 'read'::character varying, 'acknowledge'::character varying])::text[]))),
    CONSTRAINT communication_notice_events_notice_version_check CHECK ((notice_version > 0))
);

CREATE TABLE public.communication_notices (
    id uuid NOT NULL,
    source_event_id uuid NOT NULL,
    recipient_id uuid NOT NULL,
    director_designation_id uuid,
    director_version integer NOT NULL,
    decision_id uuid NOT NULL,
    project_id uuid,
    source_version bigint NOT NULL,
    decision_action character varying(30) NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    read_at timestamp with time zone,
    acknowledged_at timestamp with time zone,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    notice_kind text DEFAULT 'director'::text NOT NULL,
    title character varying(300) DEFAULT ''::character varying NOT NULL,
    link character varying(400) DEFAULT ''::character varying NOT NULL,
    source_action_id uuid,
    CONSTRAINT communication_action CHECK (((decision_action)::text = ANY ((ARRAY['approved'::character varying, 'rejected'::character varying, 'returned'::character varying, 'cancelled'::character varying, 'submit'::character varying, 'accounts_decision'::character varying, 'assigned'::character varying])::text[]))),
    CONSTRAINT communication_director_version CHECK ((director_version >= 0)),
    CONSTRAINT communication_notices_check CHECK (((acknowledged_at IS NULL) OR (read_at IS NOT NULL))),
    CONSTRAINT communication_notices_check1 CHECK ((updated_at >= created_at)),
    CONSTRAINT communication_notices_check2 CHECK (((read_at IS NULL) OR (read_at >= created_at))),
    CONSTRAINT communication_notices_check3 CHECK (((acknowledged_at IS NULL) OR (acknowledged_at >= read_at))),
    CONSTRAINT communication_notices_is_active_check CHECK (is_active),
    CONSTRAINT communication_notices_notice_kind_check CHECK ((notice_kind = ANY (ARRAY['director'::text, 'quotation'::text, 'supplier_review'::text, 'tender_manager'::text, 'procurement_manager'::text]))),
    CONSTRAINT communication_notices_source_version_check CHECK ((source_version > 0)),
    CONSTRAINT communication_notices_version_check CHECK ((version > 0)),
    CONSTRAINT communication_tender_action_kind CHECK (((notice_kind = 'tender_manager'::text) = (source_action_id IS NOT NULL)))
);

CREATE TABLE public.company_certificates (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    kind character varying(32) NOT NULL,
    certificate_number character varying(100) DEFAULT ''::character varying NOT NULL,
    issuer_name character varying(255) NOT NULL,
    amount numeric(16,2) DEFAULT 0.00 NOT NULL,
    issued_on date,
    valid_until date,
    is_active boolean DEFAULT true NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_by_id uuid,
    CONSTRAINT company_certificates_kind_check CHECK (((kind)::text = ANY ((ARRAY['solvency'::character varying, 'working_capital'::character varying, 'bank_guarantee_limit'::character varying, 'iso_certificate'::character varying])::text[])))
);

CREATE TABLE public.company_financial_years (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    financial_year character varying(16) NOT NULL,
    turnover_excluding_gst numeric(16,2) DEFAULT 0.00 NOT NULL,
    net_worth numeric(16,2) DEFAULT 0.00 NOT NULL,
    profit_after_tax numeric(16,2) DEFAULT 0.00 NOT NULL,
    is_audited boolean DEFAULT true NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_by_id uuid
);

CREATE TABLE public.company_licences (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    licence_number character varying(100) NOT NULL,
    state character varying(100) NOT NULL,
    class_grade character varying(50) DEFAULT 'Class A'::character varying NOT NULL,
    maximum_voltage_kv numeric(6,2) DEFAULT 33.00 NOT NULL,
    issued_on date,
    valid_until date NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_by_id uuid
);

CREATE TABLE public.company_profile (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    singleton boolean DEFAULT true NOT NULL,
    legal_name character varying(255) NOT NULL,
    trade_name character varying(255) DEFAULT ''::character varying NOT NULL,
    cin character varying(32) DEFAULT ''::character varying NOT NULL,
    pan character varying(16) DEFAULT ''::character varying NOT NULL,
    gstin character varying(20) DEFAULT ''::character varying NOT NULL,
    incorporation_date date,
    registered_address text DEFAULT ''::text NOT NULL,
    registered_state character varying(100) DEFAULT ''::character varying NOT NULL,
    registered_city character varying(100) DEFAULT ''::character varying NOT NULL,
    registered_pincode character varying(16) DEFAULT ''::character varying NOT NULL,
    epfo_registration character varying(50) DEFAULT ''::character varying NOT NULL,
    esic_registration character varying(50) DEFAULT ''::character varying NOT NULL,
    msme_udyam_number character varying(50) DEFAULT ''::character varying NOT NULL,
    msme_category character varying(20) DEFAULT ''::character varying NOT NULL,
    msme_activity character varying(20) DEFAULT ''::character varying NOT NULL,
    startup_recognition_number character varying(50) DEFAULT ''::character varying NOT NULL,
    iso_9001 boolean DEFAULT false NOT NULL,
    iso_14001 boolean DEFAULT false NOT NULL,
    iso_45001 boolean DEFAULT false NOT NULL,
    non_blacklisted_declaration boolean DEFAULT true NOT NULL,
    clean_insolvency_declaration boolean DEFAULT true NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_by_id uuid,
    CONSTRAINT company_profile_msme_activity_check CHECK (((msme_activity)::text = ANY ((ARRAY[''::character varying, 'manufacturing'::character varying, 'services'::character varying, 'trading'::character varying])::text[]))),
    CONSTRAINT company_profile_msme_category_check CHECK (((msme_category)::text = ANY ((ARRAY[''::character varying, 'micro'::character varying, 'small'::character varying, 'medium'::character varying])::text[]))),
    CONSTRAINT company_profile_singleton_check CHECK ((singleton = true))
);

CREATE TABLE public.company_work_experience (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    project_title character varying(255) NOT NULL,
    client_name character varying(255) NOT NULL,
    client_type character varying(32) DEFAULT 'psu'::character varying NOT NULL,
    scope_description text DEFAULT ''::text NOT NULL,
    executed_value_excluding_gst numeric(16,2) DEFAULT 0.00 NOT NULL,
    state character varying(100) DEFAULT ''::character varying NOT NULL,
    voltage_kv numeric(6,2) DEFAULT NULL::numeric,
    capacity_description character varying(100) DEFAULT ''::character varying NOT NULL,
    company_share_percent numeric(5,2) DEFAULT 100.00 NOT NULL,
    completed_on date NOT NULL,
    completion_certificate_ref character varying(100) DEFAULT ''::character varying NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_by_id uuid,
    CONSTRAINT company_work_experience_client_type_check CHECK (((client_type)::text = ANY ((ARRAY['central_govt'::character varying, 'state_govt'::character varying, 'psu'::character varying, 'public_ltd'::character varying, 'private'::character varying])::text[])))
);

CREATE TABLE public.configuration_templates (
    id uuid NOT NULL,
    subcategory_id uuid NOT NULL,
    template_name character varying(255) NOT NULL,
    model_required boolean NOT NULL,
    revision integer NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by_id uuid,
    legacy_definition jsonb NOT NULL,
    source_fingerprint text NOT NULL,
    CONSTRAINT configuration_templates_revision_check CHECK ((revision > 0)),
    CONSTRAINT configuration_templates_version_check CHECK ((version > 0))
);

CREATE TABLE public.document_package_events (
    id uuid NOT NULL,
    receipt_id uuid NOT NULL,
    file_id uuid,
    action text NOT NULL,
    version bigint NOT NULL,
    error_code text,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    CONSTRAINT document_package_events_version_check CHECK ((version > 0))
);

CREATE TABLE public.document_package_files (
    id uuid NOT NULL,
    receipt_id uuid NOT NULL,
    client_file_key uuid NOT NULL,
    filename text NOT NULL,
    kind text NOT NULL,
    declared_size bigint NOT NULL,
    declared_sha256 text NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    state text DEFAULT 'declared'::text NOT NULL,
    object_key uuid,
    upload_key uuid,
    upload_input_version bigint,
    claim_id uuid,
    lease_until timestamp with time zone,
    error_code text,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT document_package_files_check CHECK (((kind = 'zip'::text) OR (declared_size <= 33554432))),
    CONSTRAINT document_package_files_check1 CHECK (((claim_id IS NULL) = (lease_until IS NULL))),
    CONSTRAINT document_package_files_check2 CHECK (((state <> ALL (ARRAY['received'::text, 'ready'::text])) OR (object_key IS NOT NULL))),
    CONSTRAINT document_package_files_declared_sha256_check CHECK ((declared_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT document_package_files_declared_size_check CHECK (((declared_size >= 1) AND (declared_size <= 67108864))),
    CONSTRAINT document_package_files_filename_check CHECK (((length(filename) >= 1) AND (length(filename) <= 200))),
    CONSTRAINT document_package_files_kind_check CHECK ((kind = ANY (ARRAY['pdf'::text, 'docx'::text, 'doc'::text, 'xlsx'::text, 'xls'::text, 'zip'::text]))),
    CONSTRAINT document_package_files_state_check CHECK ((state = ANY (ARRAY['declared'::text, 'uploading'::text, 'received'::text, 'ready'::text, 'failed'::text]))),
    CONSTRAINT document_package_files_version_check CHECK (((version >= 1) AND (version <= '9007199254740991'::bigint)))
);

CREATE TABLE public.document_package_manifest (
    id uuid NOT NULL,
    receipt_id uuid NOT NULL,
    file_id uuid NOT NULL,
    claim_id uuid NOT NULL,
    path text NOT NULL,
    disposition text NOT NULL,
    error_code text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT document_package_manifest_disposition_check CHECK ((disposition = ANY (ARRAY['listed'::text, 'verified'::text, 'directory'::text, 'blocked'::text, 'ignored'::text])))
);

CREATE TABLE public.document_package_members (
    id uuid NOT NULL,
    receipt_id uuid NOT NULL,
    file_id uuid NOT NULL,
    path text NOT NULL,
    normalized_path text NOT NULL,
    filename text NOT NULL,
    kind text NOT NULL,
    byte_size bigint NOT NULL,
    expanded_bytes bigint NOT NULL,
    sha256 text NOT NULL,
    reader_payload bytea NOT NULL,
    reader_payload_sha256 text NOT NULL,
    validation_claim_id uuid NOT NULL,
    scanner_revision text NOT NULL,
    pages integer DEFAULT 0 NOT NULL,
    sheets integer DEFAULT 0 NOT NULL,
    cells bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    CONSTRAINT document_package_members_byte_size_check CHECK (((byte_size >= 1) AND (byte_size <= 33554432))),
    CONSTRAINT document_package_members_cells_check CHECK (((cells >= 0) AND (cells <= 100000))),
    CONSTRAINT document_package_members_check CHECK ((octet_length(reader_payload) = (byte_size + 28))),
    CONSTRAINT document_package_members_expanded_bytes_check CHECK (((expanded_bytes >= 1) AND (expanded_bytes <= 262144000))),
    CONSTRAINT document_package_members_filename_check CHECK (((length(filename) >= 1) AND (length(filename) <= 200))),
    CONSTRAINT document_package_members_kind_check CHECK ((kind = ANY (ARRAY['pdf'::text, 'docx'::text, 'xlsx'::text]))),
    CONSTRAINT document_package_members_pages_check CHECK (((pages >= 0) AND (pages <= 1000))),
    CONSTRAINT document_package_members_path_check CHECK (((length(path) >= 1) AND (length(path) <= 2048))),
    CONSTRAINT document_package_members_reader_payload_sha256_check CHECK ((reader_payload_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT document_package_members_sha256_check CHECK ((sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT document_package_members_sheets_check CHECK (((sheets >= 0) AND (sheets <= 100)))
);

CREATE TABLE public.document_package_objects (
    id uuid NOT NULL,
    receipt_id uuid NOT NULL,
    file_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    purged_at timestamp with time zone,
    purge_claim_id uuid,
    purge_lease_until timestamp with time zone
);

CREATE TABLE public.document_package_operations (
    actor_id uuid NOT NULL,
    command text NOT NULL,
    context_key text NOT NULL,
    idempotency_key uuid NOT NULL,
    fingerprint text NOT NULL,
    receipt_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT document_package_operations_fingerprint_check CHECK ((fingerprint ~ '^[0-9a-f]{64}$'::text))
);

CREATE TABLE public.document_package_receipts (
    id uuid NOT NULL,
    owner_id uuid NOT NULL,
    purpose text DEFAULT 'tender_capture'::text NOT NULL,
    state text DEFAULT 'receiving'::text NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    limit_profile_revision bigint DEFAULT 1 NOT NULL,
    expires_at timestamp with time zone DEFAULT (clock_timestamp() + '24:00:00'::interval) NOT NULL,
    consumed_by_tender_id uuid,
    claim_id uuid,
    lease_until timestamp with time zone,
    attempts integer DEFAULT 0 NOT NULL,
    error_code text,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    draft_id uuid,
    published_at timestamp with time zone,
    CONSTRAINT document_package_receipts_attempts_check CHECK (((attempts >= 0) AND (attempts <= 3))),
    CONSTRAINT document_package_receipts_check CHECK (((state = 'consumed'::text) = ((consumed_by_tender_id IS NOT NULL) OR ((draft_id IS NOT NULL) AND (published_at IS NOT NULL))))),
    CONSTRAINT document_package_receipts_check1 CHECK (((lease_until IS NULL) OR (claim_id IS NOT NULL))),
    CONSTRAINT document_package_receipts_limit_profile_revision_check CHECK ((limit_profile_revision = 1)),
    CONSTRAINT document_package_receipts_purpose_check CHECK ((purpose = 'tender_capture'::text)),
    CONSTRAINT document_package_receipts_state_check CHECK ((state = ANY (ARRAY['receiving'::text, 'verifying'::text, 'ready'::text, 'failed'::text, 'expired'::text, 'consumed'::text]))),
    CONSTRAINT document_package_receipts_version_check CHECK (((version >= 1) AND (version <= '9007199254740991'::bigint))),
    CONSTRAINT package_single_target CHECK ((NOT ((consumed_by_tender_id IS NOT NULL) AND (draft_id IS NOT NULL))))
);

CREATE TABLE public.document_reading_events (
    id uuid NOT NULL,
    document_id uuid NOT NULL,
    action character varying(40) NOT NULL,
    version integer NOT NULL,
    actor_id uuid,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);

CREATE TABLE public.document_reading_results (
    document_id uuid NOT NULL,
    result bytea NOT NULL,
    result_sha256 character varying(64) NOT NULL,
    reader_version character varying(80) NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid
);

CREATE TABLE public.document_readings (
    id uuid NOT NULL,
    module character varying(64) NOT NULL,
    entity_id uuid NOT NULL,
    filename character varying(200) NOT NULL,
    format character varying(8) NOT NULL,
    size_bytes bigint NOT NULL,
    sha256 character varying(64) NOT NULL,
    original bytea NOT NULL,
    status character varying(16) DEFAULT 'queued'::character varying NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    claim_id uuid,
    lease_until timestamp with time zone,
    error_message character varying(300) DEFAULT ''::character varying NOT NULL,
    segment_count integer DEFAULT 0 NOT NULL,
    warnings jsonb DEFAULT '[]'::jsonb NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT document_readings_attempts_check CHECK (((attempts >= 0) AND (attempts <= 3))),
    CONSTRAINT document_readings_format_check CHECK (((format)::text = ANY ((ARRAY['pdf'::character varying, 'xlsx'::character varying, 'xls'::character varying, 'docx'::character varying, 'doc'::character varying])::text[]))),
    CONSTRAINT document_readings_size_bytes_check CHECK (((size_bytes >= 1) AND (size_bytes <= 33554432))),
    CONSTRAINT document_readings_status_check CHECK (((status)::text = ANY ((ARRAY['queued'::character varying, 'processing'::character varying, 'ready'::character varying, 'failed'::character varying])::text[])))
);

CREATE TABLE public.epcerp_schema_identity (
    singleton boolean DEFAULT true NOT NULL,
    project text NOT NULL,
    CONSTRAINT epcerp_schema_identity_project_check CHECK ((project = 'epcERP'::text)),
    CONSTRAINT epcerp_schema_identity_singleton_check CHECK (singleton)
);

CREATE TABLE public.finance_accounts_acceptance (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    package_id uuid NOT NULL,
    engineer_id uuid NOT NULL,
    engineer_version integer NOT NULL,
    version integer NOT NULL,
    revision integer NOT NULL,
    status character varying(20) NOT NULL,
    source_snapshot jsonb NOT NULL,
    data jsonb NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT finance_accounts_acceptance_data_check CHECK ((jsonb_typeof(data) = 'object'::text)),
    CONSTRAINT finance_accounts_acceptance_engineer_version_check CHECK ((engineer_version > 0)),
    CONSTRAINT finance_accounts_acceptance_revision_check CHECK ((revision > 0)),
    CONSTRAINT finance_accounts_acceptance_source_snapshot_check CHECK ((jsonb_typeof(source_snapshot) = 'object'::text)),
    CONSTRAINT finance_accounts_acceptance_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'accepted'::character varying])::text[]))),
    CONSTRAINT finance_accounts_acceptance_version_check CHECK ((version > 0))
);

CREATE TABLE public.finance_accounts_acceptance_events (
    id uuid NOT NULL,
    subject_id uuid NOT NULL,
    action character varying(20) NOT NULL,
    reason text NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    CONSTRAINT finance_accounts_acceptance_events_action_check CHECK (((action)::text = ANY ((ARRAY['record'::character varying, 'revise'::character varying])::text[]))),
    CONSTRAINT finance_accounts_acceptance_events_reason_check CHECK (((length(TRIM(BOTH FROM reason)) >= 1) AND (length(TRIM(BOTH FROM reason)) <= 2000)))
);

CREATE TABLE public.finance_advance_decisions (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    package_id uuid NOT NULL,
    request_id uuid NOT NULL,
    order_id uuid NOT NULL,
    order_version integer NOT NULL,
    source_version integer NOT NULL,
    submitted_by_id uuid NOT NULL,
    outcome character varying(20) NOT NULL,
    kind character varying(24) NOT NULL,
    amount numeric(16,2) NOT NULL,
    currency character(3) NOT NULL,
    source_fingerprint character(64) NOT NULL,
    reason character varying(2000) NOT NULL,
    is_active boolean NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid NOT NULL,
    updated_by_id uuid NOT NULL,
    CONSTRAINT finance_advance_decisions_amount_check CHECK ((amount >= (0)::numeric)),
    CONSTRAINT finance_advance_decisions_check CHECK ((created_by_id <> submitted_by_id)),
    CONSTRAINT finance_advance_decisions_check1 CHECK ((updated_at = created_at)),
    CONSTRAINT finance_advance_decisions_check2 CHECK ((updated_by_id = created_by_id)),
    CONSTRAINT finance_advance_decisions_check3 CHECK (((((kind)::text = 'payment'::text) AND (amount > (0)::numeric)) OR (((kind)::text = 'not_applicable'::text) AND (amount = (0)::numeric)))),
    CONSTRAINT finance_advance_decisions_currency_check CHECK ((currency ~ '^[A-Z]{3}$'::text)),
    CONSTRAINT finance_advance_decisions_is_active_check CHECK (is_active),
    CONSTRAINT finance_advance_decisions_kind_check CHECK (((kind)::text = ANY ((ARRAY['payment'::character varying, 'not_applicable'::character varying])::text[]))),
    CONSTRAINT finance_advance_decisions_order_version_check CHECK ((order_version > 1)),
    CONSTRAINT finance_advance_decisions_outcome_check CHECK (((outcome)::text = ANY ((ARRAY['approved'::character varying, 'rejected'::character varying])::text[]))),
    CONSTRAINT finance_advance_decisions_reason_check CHECK ((length(TRIM(BOTH FROM reason)) > 0)),
    CONSTRAINT finance_advance_decisions_source_fingerprint_check CHECK ((source_fingerprint ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT finance_advance_decisions_source_version_check CHECK ((source_version > 1))
);

CREATE TABLE public.finance_budget_decisions (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    source_version integer NOT NULL,
    source_revision integer NOT NULL,
    amount numeric(16,2) NOT NULL,
    currency character(3) NOT NULL,
    source_fingerprint character(64) NOT NULL,
    reason character varying(2000) NOT NULL,
    is_active boolean NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT finance_budget_decisions_amount_check CHECK ((amount > (0)::numeric)),
    CONSTRAINT finance_budget_decisions_check CHECK ((updated_at = created_at)),
    CONSTRAINT finance_budget_decisions_check1 CHECK ((NOT (updated_by_id IS DISTINCT FROM created_by_id))),
    CONSTRAINT finance_budget_decisions_currency_check CHECK ((currency ~ '^[A-Z]{3}$'::text)),
    CONSTRAINT finance_budget_decisions_is_active_check CHECK (is_active),
    CONSTRAINT finance_budget_decisions_reason_check CHECK ((length(btrim((reason)::text)) > 0)),
    CONSTRAINT finance_budget_decisions_source_fingerprint_check CHECK ((source_fingerprint ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT finance_budget_decisions_source_revision_check CHECK ((source_revision > 0)),
    CONSTRAINT finance_budget_decisions_source_version_check CHECK ((source_version > 0))
);

CREATE TABLE public.finance_quotation_decisions (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    bo_id uuid NOT NULL,
    attempt_id uuid NOT NULL,
    source_version integer NOT NULL,
    submitted_by_id uuid NOT NULL,
    outcome character varying(20) NOT NULL,
    amount numeric(16,2) NOT NULL,
    currency character(3) NOT NULL,
    source_fingerprint character(64) NOT NULL,
    reason character varying(2000) NOT NULL,
    is_active boolean NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT finance_quotation_decisions_amount_check CHECK ((amount >= (0)::numeric)),
    CONSTRAINT finance_quotation_decisions_check CHECK ((created_by_id IS DISTINCT FROM submitted_by_id)),
    CONSTRAINT finance_quotation_decisions_check1 CHECK ((updated_at = created_at)),
    CONSTRAINT finance_quotation_decisions_check2 CHECK ((NOT (updated_by_id IS DISTINCT FROM created_by_id))),
    CONSTRAINT finance_quotation_decisions_currency_check CHECK ((currency ~ '^[A-Z]{3}$'::text)),
    CONSTRAINT finance_quotation_decisions_is_active_check CHECK (is_active),
    CONSTRAINT finance_quotation_decisions_outcome_check CHECK (((outcome)::text = ANY ((ARRAY['approved'::character varying, 'rejected'::character varying])::text[]))),
    CONSTRAINT finance_quotation_decisions_reason_check CHECK ((length(btrim((reason)::text)) > 0)),
    CONSTRAINT finance_quotation_decisions_source_fingerprint_check CHECK ((source_fingerprint ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT finance_quotation_decisions_source_version_check CHECK ((source_version > 0))
);

CREATE TABLE public.indian_cities (
    state_id uuid NOT NULL,
    id uuid NOT NULL,
    name character varying(128) NOT NULL,
    normalized_name character varying(128) NOT NULL,
    code character varying(32) NOT NULL,
    is_active boolean NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by_id uuid
);

CREATE TABLE public.indian_states (
    id uuid NOT NULL,
    name character varying(128) NOT NULL,
    normalized_name character varying(128) NOT NULL,
    code character varying(32) NOT NULL,
    is_active boolean NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by_id uuid
);

CREATE TABLE public.industries (
    id uuid NOT NULL,
    name character varying(128) NOT NULL,
    normalized_name character varying(128) NOT NULL,
    code character varying(32) NOT NULL,
    is_active boolean NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by_id uuid
);

CREATE TABLE public.manufacturer_imports (
    source_id uuid NOT NULL,
    go_id uuid NOT NULL,
    source_snapshot jsonb NOT NULL,
    source_fingerprint text NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid
);

CREATE TABLE public.manufacturers (
    id uuid NOT NULL,
    code character varying(32) NOT NULL,
    name character varying(255) NOT NULL,
    data jsonb NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by_id uuid,
    CONSTRAINT manufacturers_version_positive CHECK ((version > 0))
);

CREATE TABLE public.manufacturers_events (
    id uuid NOT NULL,
    subject_id uuid NOT NULL,
    action character varying(40) NOT NULL,
    reason character varying(2000) NOT NULL,
    before jsonb NOT NULL,
    after jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid
);

CREATE TABLE public.material_catalogue (
    id uuid NOT NULL,
    code character varying(32) NOT NULL,
    name character varying(255) NOT NULL,
    data jsonb NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by_id uuid,
    CONSTRAINT material_catalogue_version_positive CHECK ((version > 0))
);

CREATE TABLE public.organisation_director_designation (
    slot boolean DEFAULT true NOT NULL,
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    version integer NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT organisation_director_designation_slot_check CHECK (slot),
    CONSTRAINT organisation_director_designation_version_check CHECK ((version > 0))
);

CREATE TABLE public.organisation_director_events (
    id uuid NOT NULL,
    designation_id uuid NOT NULL,
    action character varying(30) NOT NULL,
    reason character varying(2000) NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    CONSTRAINT organisation_director_events_action_check CHECK (((action)::text = ANY ((ARRAY['designate'::character varying, 'change'::character varying, 'revoke'::character varying, 'reactivate'::character varying])::text[]))),
    CONSTRAINT organisation_director_events_reason_check CHECK ((length(TRIM(BOTH FROM reason)) > 0))
);

CREATE TABLE public.project_budget_allocations (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    sort_order integer NOT NULL,
    bo_id uuid NOT NULL,
    line_id uuid NOT NULL,
    head_id uuid NOT NULL,
    amount numeric(16,2) NOT NULL,
    owner_id uuid NOT NULL,
    is_active boolean NOT NULL,
    unit_price numeric(16,2),
    pricing_quantity numeric(18,4),
    source_snapshot jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT project_budget_allocations_amount_check CHECK ((amount >= (0)::numeric)),
    CONSTRAINT project_budget_allocations_check CHECK (((unit_price IS NULL) = (pricing_quantity IS NULL))),
    CONSTRAINT project_budget_allocations_pricing_quantity_check CHECK ((pricing_quantity > (0)::numeric)),
    CONSTRAINT project_budget_allocations_sort_order_check CHECK ((sort_order >= 0)),
    CONSTRAINT project_budget_allocations_unit_price_check CHECK ((unit_price >= (0)::numeric))
);

CREATE TABLE public.project_budget_events (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    action character varying(30) NOT NULL,
    reason character varying(2000) NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    CONSTRAINT project_budget_events_action_check CHECK (((action)::text = ANY ((ARRAY['save'::character varying, 'price_items'::character varying, 'approve'::character varying, 'revise'::character varying])::text[])))
);

CREATE TABLE public.project_budget_heads (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    sort_order integer NOT NULL,
    parent_id uuid,
    name character varying(160) NOT NULL,
    kind character varying(30) NOT NULL,
    amount numeric(16,2) NOT NULL,
    owner_id uuid NOT NULL,
    is_active boolean NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT project_budget_heads_amount_check CHECK ((amount >= (0)::numeric)),
    CONSTRAINT project_budget_heads_sort_order_check CHECK ((sort_order >= 0))
);

CREATE TABLE public.project_budgets (
    project_id uuid NOT NULL,
    version integer NOT NULL,
    revision integer NOT NULL,
    state character varying(20) NOT NULL,
    total numeric(16,2) NOT NULL,
    currency character(3) NOT NULL,
    owner_id uuid NOT NULL,
    notes character varying(2000) NOT NULL,
    pricing_mode character varying(20) NOT NULL,
    is_active boolean NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    approval_decision_id uuid,
    approved_snapshot jsonb DEFAULT 'null'::jsonb NOT NULL,
    CONSTRAINT project_budget_approval_snapshot CHECK ((((approval_decision_id IS NULL) = (approved_snapshot = 'null'::jsonb)) AND (((state)::text <> 'approved'::text) OR (approval_decision_id IS NOT NULL)))),
    CONSTRAINT project_budgets_pricing_mode_check CHECK (((pricing_mode)::text = ANY ((ARRAY[''::character varying, 'unit_rates'::character varying])::text[]))),
    CONSTRAINT project_budgets_revision_check CHECK ((revision > 0)),
    CONSTRAINT project_budgets_state_check CHECK (((state)::text = ANY ((ARRAY['draft'::character varying, 'approved'::character varying])::text[]))),
    CONSTRAINT project_budgets_total_check CHECK ((total >= (0)::numeric)),
    CONSTRAINT project_budgets_version_check CHECK ((version > 0))
);

CREATE TABLE public.project_imports (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    source_id uuid NOT NULL,
    source_sha256 text NOT NULL,
    source_snapshot jsonb NOT NULL,
    source_created_at timestamp with time zone NOT NULL,
    source_updated_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    CONSTRAINT project_imports_source_sha256_check CHECK ((length(source_sha256) = 64))
);

CREATE TABLE public.project_manager_history (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    previous_manager_id uuid,
    project_manager_id uuid NOT NULL,
    project_version integer NOT NULL,
    reason character varying(2000) NOT NULL,
    created_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    CONSTRAINT project_manager_history_check CHECK (((previous_manager_id IS NULL) OR (previous_manager_id <> project_manager_id))),
    CONSTRAINT project_manager_history_project_version_check CHECK ((project_version > 0)),
    CONSTRAINT project_manager_history_reason_check CHECK ((length(btrim((reason)::text)) > 0))
);

CREATE TABLE public.project_material_events (
    id uuid NOT NULL,
    subject_id uuid NOT NULL,
    action character varying(40) NOT NULL,
    reason character varying(2000) NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid
);

CREATE TABLE public.project_material_requirements (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    item_id uuid NOT NULL,
    engineering_key character varying(64) NOT NULL,
    item_snapshot jsonb NOT NULL,
    quantity numeric(16,3) NOT NULL,
    required_date date NOT NULL,
    allocations jsonb NOT NULL,
    alternatives jsonb NOT NULL,
    is_active boolean NOT NULL,
    version integer NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT project_material_requirements_quantity_check CHECK ((quantity > (0)::numeric)),
    CONSTRAINT project_material_requirements_version_check CHECK ((version > 0))
);

CREATE TABLE public.project_metadata_changes (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    project_version integer NOT NULL,
    reason text NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    CONSTRAINT project_metadata_changes_project_version_check CHECK ((project_version > 1)),
    CONSTRAINT project_metadata_changes_reason_check CHECK (((length(TRIM(BOTH FROM reason)) >= 1) AND (length(TRIM(BOTH FROM reason)) <= 2000)))
);

CREATE SEQUENCE public.project_number_sequence
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE TABLE public.project_responsibility_events (
    id uuid NOT NULL,
    profile_id uuid NOT NULL,
    project_id uuid NOT NULL,
    action character varying(20) NOT NULL,
    reason character varying(2000) NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    CONSTRAINT project_responsibility_events_action_check CHECK (((action)::text = ANY ((ARRAY['assign'::character varying, 'revise'::character varying, 'revoke'::character varying])::text[]))),
    CONSTRAINT project_responsibility_events_reason_check CHECK ((length(btrim((reason)::text)) > 0))
);

CREATE TABLE public.project_responsibility_profiles (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    user_id uuid NOT NULL,
    bundle_id uuid NOT NULL,
    responsibility character varying(80) NOT NULL,
    responsibility_key character varying(80) NOT NULL,
    permission_codes jsonb NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    version integer NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT project_responsibility_profiles_permission_codes_check CHECK (((jsonb_typeof(permission_codes) = 'array'::text) AND ((jsonb_array_length(permission_codes) >= 1) AND (jsonb_array_length(permission_codes) <= 100)))),
    CONSTRAINT project_responsibility_profiles_responsibility_check CHECK ((length(btrim((responsibility)::text)) > 0)),
    CONSTRAINT project_responsibility_profiles_version_check CHECK ((version > 0))
);

CREATE TABLE public.project_schedule_changes (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    project_version integer NOT NULL,
    before_calculation jsonb,
    after_calculation jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid
);

CREATE TABLE public.projects (
    id uuid NOT NULL,
    project_number character varying(32) NOT NULL,
    name character varying(255) NOT NULL,
    normalized_name character varying(255) NOT NULL,
    client_id uuid NOT NULL,
    project_manager_id uuid NOT NULL,
    lifecycle_status character varying(16) DEFAULT 'draft'::character varying NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_by_id uuid,
    short_name text DEFAULT ''::text NOT NULL,
    project_type text DEFAULT ''::text NOT NULL,
    site_name text DEFAULT ''::text NOT NULL,
    site_state text DEFAULT ''::text NOT NULL,
    site_city text DEFAULT ''::text NOT NULL,
    scope_summary text DEFAULT ''::text NOT NULL,
    operations_manager_id uuid,
    contract_value numeric(16,2),
    currency text DEFAULT ''::text NOT NULL,
    loi_number text DEFAULT ''::text NOT NULL,
    po_number text DEFAULT ''::text NOT NULL,
    schedule_start_basis text DEFAULT ''::text NOT NULL,
    planned_start_date date,
    planned_completion_date date,
    loi_date date,
    po_date date,
    actual_start_date date,
    site_handover_date date,
    planned_duration_days integer,
    safety_buffer_percent numeric(5,2),
    safety_buffer_days integer,
    remarks text DEFAULT ''::text NOT NULL,
    actual_end_date date,
    schedule_calculation jsonb,
    purchase_enabled boolean DEFAULT false NOT NULL,
    CONSTRAINT project_capture_bounds CHECK ((((contract_value IS NULL) OR (contract_value >= (0)::numeric)) AND ((planned_duration_days IS NULL) OR ((planned_duration_days >= 0) AND (planned_duration_days <= 36500))) AND ((safety_buffer_percent IS NULL) OR ((safety_buffer_percent >= (0)::numeric) AND (safety_buffer_percent <= (100)::numeric))) AND ((safety_buffer_days IS NULL) OR ((safety_buffer_days >= 0) AND (safety_buffer_days <= 36500))))),
    CONSTRAINT projects_lifecycle_status_check CHECK (((lifecycle_status)::text = ANY ((ARRAY['draft'::character varying, 'active'::character varying, 'on_hold'::character varying, 'closed'::character varying])::text[]))),
    CONSTRAINT projects_version_check CHECK ((version > 0))
);

CREATE TABLE public.purchase_acceptance_events (
    id uuid NOT NULL,
    subject_id uuid NOT NULL,
    action character varying(20) NOT NULL,
    reason text NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    CONSTRAINT purchase_acceptance_events_action_check CHECK (((action)::text = 'record'::text)),
    CONSTRAINT purchase_acceptance_events_reason_check CHECK (((length(TRIM(BOTH FROM reason)) >= 1) AND (length(TRIM(BOTH FROM reason)) <= 2000)))
);

CREATE TABLE public.purchase_acceptances (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    package_id uuid NOT NULL,
    order_id uuid NOT NULL,
    order_version integer NOT NULL,
    version integer NOT NULL,
    status character varying(32) NOT NULL,
    order_snapshot jsonb NOT NULL,
    data jsonb NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_acceptances_data_check CHECK ((jsonb_typeof(data) = 'object'::text)),
    CONSTRAINT purchase_acceptances_order_snapshot_check CHECK ((jsonb_typeof(order_snapshot) = 'object'::text)),
    CONSTRAINT purchase_acceptances_order_version_check CHECK ((order_version > 1)),
    CONSTRAINT purchase_acceptances_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'accepted'::character varying, 'accepted_with_exceptions'::character varying])::text[]))),
    CONSTRAINT purchase_acceptances_version_check CHECK ((version > 0))
);

CREATE TABLE public.purchase_advance_events (
    id uuid NOT NULL,
    subject_id uuid NOT NULL,
    action character varying(20) NOT NULL,
    reason character varying(2000) NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL,
    created_by_id uuid NOT NULL,
    CONSTRAINT purchase_advance_events_action_check CHECK (((action)::text = ANY ((ARRAY['prepare'::character varying, 'submit'::character varying, 'revise'::character varying, 'approve'::character varying, 'reject'::character varying, 'disburse'::character varying])::text[]))),
    CONSTRAINT purchase_advance_events_reason_check CHECK ((length(TRIM(BOTH FROM reason)) > 0))
);

CREATE TABLE public.purchase_advances (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    package_id uuid NOT NULL,
    order_id uuid NOT NULL,
    order_version integer NOT NULL,
    version integer NOT NULL,
    state character varying(24) NOT NULL,
    source_snapshot jsonb NOT NULL,
    data jsonb NOT NULL,
    submitted_snapshot text NOT NULL,
    source_fingerprint character varying(64) NOT NULL,
    decision_id uuid,
    is_active boolean NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid NOT NULL,
    updated_by_id uuid NOT NULL,
    CONSTRAINT purchase_advances_data_check CHECK ((jsonb_typeof(data) = 'object'::text)),
    CONSTRAINT purchase_advances_is_active_check CHECK (is_active),
    CONSTRAINT purchase_advances_order_version_check CHECK ((order_version > 1)),
    CONSTRAINT purchase_advances_source_snapshot_check CHECK ((jsonb_typeof(source_snapshot) = 'object'::text)),
    CONSTRAINT purchase_advances_state_check CHECK (((state)::text = ANY ((ARRAY['draft'::character varying, 'requested'::character varying, 'approved'::character varying, 'rejected'::character varying, 'not_applicable'::character varying, 'disbursed'::character varying])::text[]))),
    CONSTRAINT purchase_advances_version_check CHECK ((version > 0))
);

CREATE TABLE public.purchase_award_handoffs (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    bo_id uuid NOT NULL,
    attempt_id uuid NOT NULL,
    packages jsonb NOT NULL,
    reason text NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_award_handoffs_packages_check CHECK (((jsonb_typeof(packages) = 'array'::text) AND (jsonb_array_length(packages) > 0))),
    CONSTRAINT purchase_award_handoffs_reason_check CHECK (((length(TRIM(BOTH FROM reason)) >= 1) AND (length(TRIM(BOTH FROM reason)) <= 2000)))
);

CREATE TABLE public.purchase_bo_events (
    id uuid NOT NULL,
    subject_id uuid NOT NULL,
    action character varying(40) NOT NULL,
    reason character varying(2000) NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid
);

CREATE TABLE public.purchase_bo_lines (
    id uuid NOT NULL,
    bo_id uuid NOT NULL,
    item_id uuid,
    quantity numeric(16,3),
    required_date date,
    reference character varying(500) NOT NULL,
    notes character varying(4000) NOT NULL,
    item_snapshot jsonb NOT NULL,
    demand_id uuid,
    sort_order integer NOT NULL,
    is_active boolean NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_bo_lines_quantity_check CHECK ((quantity > (0)::numeric))
);

CREATE TABLE public.purchase_bo_lists (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    preparer_id uuid NOT NULL,
    name character varying(128) NOT NULL,
    state character varying(20) NOT NULL,
    is_active boolean NOT NULL,
    version integer NOT NULL,
    completed_at timestamp with time zone,
    completed_by_id uuid,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    merged_into_id uuid,
    CONSTRAINT purchase_bo_lists_check CHECK ((((state)::text = 'completed'::text) = ((completed_at IS NOT NULL) AND (completed_by_id IS NOT NULL)))),
    CONSTRAINT purchase_bo_lists_state_check CHECK (((state)::text = ANY ((ARRAY['draft'::character varying, 'completed'::character varying])::text[]))),
    CONSTRAINT purchase_bo_lists_version_check CHECK ((version > 0))
);

CREATE TABLE public.purchase_client_clearance_events (
    id uuid NOT NULL,
    subject_id uuid NOT NULL,
    action character varying(20) NOT NULL,
    reason text NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    CONSTRAINT purchase_client_clearance_events_action_check CHECK (((action)::text = ANY ((ARRAY['record'::character varying, 'revise'::character varying])::text[]))),
    CONSTRAINT purchase_client_clearance_events_reason_check CHECK (((length(TRIM(BOTH FROM reason)) >= 1) AND (length(TRIM(BOTH FROM reason)) <= 2000)))
);

CREATE TABLE public.purchase_client_clearances (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    package_id uuid NOT NULL,
    quality_id uuid NOT NULL,
    quality_version integer NOT NULL,
    version integer NOT NULL,
    revision integer NOT NULL,
    status character varying(20) NOT NULL,
    source_snapshot jsonb NOT NULL,
    data jsonb NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_client_clearances_data_check CHECK ((jsonb_typeof(data) = 'object'::text)),
    CONSTRAINT purchase_client_clearances_quality_version_check CHECK ((quality_version > 0)),
    CONSTRAINT purchase_client_clearances_revision_check CHECK ((revision > 0)),
    CONSTRAINT purchase_client_clearances_source_snapshot_check CHECK ((jsonb_typeof(source_snapshot) = 'object'::text)),
    CONSTRAINT purchase_client_clearances_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'cleared'::character varying])::text[]))),
    CONSTRAINT purchase_client_clearances_version_check CHECK ((version > 0))
);

CREATE TABLE public.purchase_dispatch_events (
    id uuid NOT NULL,
    subject_id uuid NOT NULL,
    action character varying(20) NOT NULL,
    reason text NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    CONSTRAINT purchase_dispatch_events_action_check CHECK (((action)::text = ANY ((ARRAY['record'::character varying, 'revise'::character varying])::text[]))),
    CONSTRAINT purchase_dispatch_events_reason_check CHECK (((length(TRIM(BOTH FROM reason)) >= 1) AND (length(TRIM(BOTH FROM reason)) <= 2000)))
);

CREATE TABLE public.purchase_dispatches (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    package_id uuid NOT NULL,
    clearance_id uuid NOT NULL,
    clearance_version integer NOT NULL,
    version integer NOT NULL,
    revision integer NOT NULL,
    status character varying(20) NOT NULL,
    source_snapshot jsonb NOT NULL,
    data jsonb NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_dispatches_clearance_version_check CHECK ((clearance_version > 0)),
    CONSTRAINT purchase_dispatches_data_check CHECK ((jsonb_typeof(data) = 'object'::text)),
    CONSTRAINT purchase_dispatches_revision_check CHECK ((revision > 0)),
    CONSTRAINT purchase_dispatches_source_snapshot_check CHECK ((jsonb_typeof(source_snapshot) = 'object'::text)),
    CONSTRAINT purchase_dispatches_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'dispatched'::character varying])::text[]))),
    CONSTRAINT purchase_dispatches_version_check CHECK ((version > 0))
);

CREATE TABLE public.purchase_engineer_confirmations (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    package_id uuid NOT NULL,
    grn_id uuid NOT NULL,
    grn_version integer NOT NULL,
    version integer NOT NULL,
    revision integer NOT NULL,
    status character varying(20) NOT NULL,
    source_snapshot jsonb NOT NULL,
    data jsonb NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_engineer_confirmations_data_check CHECK ((jsonb_typeof(data) = 'object'::text)),
    CONSTRAINT purchase_engineer_confirmations_grn_version_check CHECK ((grn_version > 0)),
    CONSTRAINT purchase_engineer_confirmations_revision_check CHECK ((revision > 0)),
    CONSTRAINT purchase_engineer_confirmations_source_snapshot_check CHECK ((jsonb_typeof(source_snapshot) = 'object'::text)),
    CONSTRAINT purchase_engineer_confirmations_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'confirmed'::character varying])::text[]))),
    CONSTRAINT purchase_engineer_confirmations_version_check CHECK ((version > 0))
);

CREATE TABLE public.purchase_engineer_confirmations_events (
    id uuid NOT NULL,
    subject_id uuid NOT NULL,
    action character varying(20) NOT NULL,
    reason text NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    CONSTRAINT purchase_engineer_confirmations_events_action_check CHECK (((action)::text = ANY ((ARRAY['record'::character varying, 'revise'::character varying])::text[]))),
    CONSTRAINT purchase_engineer_confirmations_events_reason_check CHECK (((length(TRIM(BOTH FROM reason)) >= 1) AND (length(TRIM(BOTH FROM reason)) <= 2000)))
);

CREATE TABLE public.purchase_fms_owner_history (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    previous_manager_id uuid,
    purchase_manager_id uuid,
    workspace_version integer NOT NULL,
    reason text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    CONSTRAINT purchase_fms_owner_history_reason_check CHECK (((length(TRIM(BOTH FROM reason)) >= 1) AND (length(TRIM(BOTH FROM reason)) <= 2000))),
    CONSTRAINT purchase_fms_owner_history_workspace_version_check CHECK ((workspace_version > 0))
);

CREATE TABLE public.purchase_fms_workspaces (
    project_id uuid NOT NULL,
    purchase_manager_id uuid,
    version integer DEFAULT 1 NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_by_id uuid,
    CONSTRAINT purchase_fms_workspaces_version_check CHECK ((version > 0))
);

CREATE TABLE public.purchase_grns (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    package_id uuid NOT NULL,
    receipt_id uuid NOT NULL,
    receipt_version integer NOT NULL,
    version integer NOT NULL,
    revision integer NOT NULL,
    status character varying(20) NOT NULL,
    source_snapshot jsonb NOT NULL,
    data jsonb NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_grns_data_check CHECK ((jsonb_typeof(data) = 'object'::text)),
    CONSTRAINT purchase_grns_receipt_version_check CHECK ((receipt_version > 0)),
    CONSTRAINT purchase_grns_revision_check CHECK ((revision > 0)),
    CONSTRAINT purchase_grns_source_snapshot_check CHECK ((jsonb_typeof(source_snapshot) = 'object'::text)),
    CONSTRAINT purchase_grns_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'issued'::character varying])::text[]))),
    CONSTRAINT purchase_grns_version_check CHECK ((version > 0))
);

CREATE TABLE public.purchase_grns_events (
    id uuid NOT NULL,
    subject_id uuid NOT NULL,
    action character varying(20) NOT NULL,
    reason text NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    CONSTRAINT purchase_grns_events_action_check CHECK (((action)::text = ANY ((ARRAY['record'::character varying, 'revise'::character varying])::text[]))),
    CONSTRAINT purchase_grns_events_reason_check CHECK (((length(TRIM(BOTH FROM reason)) >= 1) AND (length(TRIM(BOTH FROM reason)) <= 2000)))
);

CREATE TABLE public.purchase_group_quotations (
    bo_id uuid NOT NULL,
    project_id uuid NOT NULL,
    version integer NOT NULL,
    quotes jsonb NOT NULL,
    is_active boolean NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    frozen_document_ids uuid[],
    CONSTRAINT purchase_group_quotations_quotes_check CHECK ((jsonb_typeof(quotes) = 'array'::text)),
    CONSTRAINT purchase_group_quotations_version_check CHECK ((version > 0)),
    CONSTRAINT purchase_quotation_freeze_has_documents CHECK (((frozen_document_ids IS NULL) OR (cardinality(frozen_document_ids) > 0)))
);

CREATE TABLE public.purchase_hold_events (
    id uuid NOT NULL,
    subject_id uuid NOT NULL,
    action character varying(40) NOT NULL,
    reason character varying(2000) NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid
);

CREATE TABLE public.purchase_holds (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    doer_id uuid NOT NULL,
    resolver_id uuid NOT NULL,
    data jsonb NOT NULL,
    is_active boolean NOT NULL,
    version integer NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_holds_version_check CHECK ((version > 0))
);

CREATE TABLE public.purchase_indent_dispatch_notices (
    id uuid NOT NULL,
    document_id uuid NOT NULL,
    recipient_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    resolved_at timestamp with time zone,
    updated_by_id uuid
);

CREATE TABLE public.purchase_indent_document_events (
    id uuid NOT NULL,
    document_id uuid NOT NULL,
    action character varying(40) NOT NULL,
    reason character varying(2000) NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid
);

CREATE TABLE public.purchase_indent_documents (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    bo_id uuid NOT NULL,
    group_id uuid NOT NULL,
    vendor_id uuid NOT NULL,
    scope_revision integer NOT NULL,
    version integer NOT NULL,
    data jsonb NOT NULL,
    is_active boolean NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_indent_documents_scope_revision_check CHECK ((scope_revision > 0)),
    CONSTRAINT purchase_indent_documents_version_check CHECK ((version > 0))
);

CREATE TABLE public.purchase_indent_events (
    id uuid NOT NULL,
    subject_id uuid NOT NULL,
    action character varying(40) NOT NULL,
    reason character varying(2000) NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid
);

CREATE TABLE public.purchase_indent_group_entries (
    id uuid NOT NULL,
    bo_id uuid NOT NULL,
    name character varying(128) NOT NULL,
    scope_revision integer NOT NULL,
    data jsonb NOT NULL,
    is_active boolean NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_indent_group_entries_scope_revision_check CHECK ((scope_revision > 0))
);

CREATE TABLE public.purchase_indent_group_events (
    id uuid NOT NULL,
    bo_id uuid NOT NULL,
    action character varying(40) NOT NULL,
    reason character varying(2000) NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid
);

CREATE TABLE public.purchase_indent_group_sets (
    bo_id uuid NOT NULL,
    project_id uuid NOT NULL,
    version integer NOT NULL,
    data jsonb NOT NULL,
    is_active boolean NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_indent_group_sets_version_check CHECK ((version > 0))
);

CREATE TABLE public.purchase_indent_legacy_terms (
    document_id uuid NOT NULL,
    terms jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    CONSTRAINT purchase_indent_legacy_terms_terms_check CHECK (((jsonb_typeof(terms) = 'object'::text) AND ((terms ->> 'source'::text) = 'legacy_template_v1'::text)))
);

CREATE TABLE public.purchase_indent_releases (
    indent_id uuid NOT NULL,
    demand_id uuid NOT NULL,
    quantity numeric(16,3) NOT NULL,
    snapshot jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    CONSTRAINT purchase_indent_releases_quantity_check CHECK ((quantity > (0)::numeric))
);

CREATE TABLE public.purchase_indent_templates (
    category_code text NOT NULL,
    version integer NOT NULL,
    terms text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    template_id uuid DEFAULT '00000000-0000-0000-0000-000000000001'::uuid NOT NULL,
    name text DEFAULT 'Standard'::text NOT NULL,
    CONSTRAINT purchase_indent_templates_name_check CHECK (((length(btrim(name)) >= 1) AND (length(btrim(name)) <= 120))),
    CONSTRAINT purchase_indent_templates_terms_check CHECK (((length(btrim(terms)) >= 1) AND (length(btrim(terms)) <= 8000))),
    CONSTRAINT purchase_indent_templates_version_check CHECK ((version > 0))
);

CREATE TABLE public.purchase_indents (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    doer_id uuid NOT NULL,
    resolver_id uuid NOT NULL,
    data jsonb NOT NULL,
    is_active boolean NOT NULL,
    version integer NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_indents_version_check CHECK ((version > 0))
);

CREATE TABLE public.purchase_inspection_events (
    id uuid NOT NULL,
    subject_id uuid NOT NULL,
    action character varying(20) NOT NULL,
    reason text NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    CONSTRAINT purchase_inspection_events_action_check CHECK (((action)::text = ANY ((ARRAY['record'::character varying, 'revise'::character varying])::text[]))),
    CONSTRAINT purchase_inspection_events_reason_check CHECK (((length(TRIM(BOTH FROM reason)) >= 1) AND (length(TRIM(BOTH FROM reason)) <= 2000)))
);

CREATE TABLE public.purchase_inspection_report_events (
    id uuid NOT NULL,
    subject_id uuid NOT NULL,
    action character varying(20) NOT NULL,
    reason text NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    CONSTRAINT purchase_inspection_report_events_action_check CHECK (((action)::text = ANY ((ARRAY['record'::character varying, 'revise'::character varying])::text[]))),
    CONSTRAINT purchase_inspection_report_events_reason_check CHECK (((length(TRIM(BOTH FROM reason)) >= 1) AND (length(TRIM(BOTH FROM reason)) <= 2000)))
);

CREATE TABLE public.purchase_inspection_reports (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    package_id uuid NOT NULL,
    inspection_id uuid NOT NULL,
    inspection_version integer NOT NULL,
    version integer NOT NULL,
    revision integer NOT NULL,
    status character varying(20) NOT NULL,
    source_snapshot jsonb NOT NULL,
    data jsonb NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_inspection_reports_data_check CHECK ((jsonb_typeof(data) = 'object'::text)),
    CONSTRAINT purchase_inspection_reports_inspection_version_check CHECK ((inspection_version > 0)),
    CONSTRAINT purchase_inspection_reports_revision_check CHECK ((revision > 0)),
    CONSTRAINT purchase_inspection_reports_source_snapshot_check CHECK ((jsonb_typeof(source_snapshot) = 'object'::text)),
    CONSTRAINT purchase_inspection_reports_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'issued'::character varying])::text[]))),
    CONSTRAINT purchase_inspection_reports_version_check CHECK ((version > 0))
);

CREATE TABLE public.purchase_inspections (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    package_id uuid NOT NULL,
    order_id uuid NOT NULL,
    order_version integer NOT NULL,
    acceptance_id uuid NOT NULL,
    acceptance_version integer NOT NULL,
    technical_id uuid NOT NULL,
    technical_version integer NOT NULL,
    version integer NOT NULL,
    revision integer NOT NULL,
    status character varying(32) NOT NULL,
    source_snapshot jsonb NOT NULL,
    data jsonb NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_inspections_acceptance_version_check CHECK ((acceptance_version > 0)),
    CONSTRAINT purchase_inspections_data_check CHECK ((jsonb_typeof(data) = 'object'::text)),
    CONSTRAINT purchase_inspections_order_version_check CHECK ((order_version > 1)),
    CONSTRAINT purchase_inspections_revision_check CHECK ((revision > 0)),
    CONSTRAINT purchase_inspections_source_snapshot_check CHECK ((jsonb_typeof(source_snapshot) = 'object'::text)),
    CONSTRAINT purchase_inspections_status_check CHECK (((status)::text = ANY ((ARRAY['not_scheduled'::character varying, 'call_received'::character varying, 'scheduled'::character varying, 'cleared'::character varying, 'rejected'::character varying])::text[]))),
    CONSTRAINT purchase_inspections_technical_version_check CHECK ((technical_version > 0)),
    CONSTRAINT purchase_inspections_version_check CHECK ((version > 0))
);

CREATE TABLE public.purchase_loi_document_events (
    id uuid NOT NULL,
    document_id uuid NOT NULL,
    action text NOT NULL,
    created_at timestamp with time zone NOT NULL,
    created_by_id uuid NOT NULL,
    CONSTRAINT purchase_loi_document_events_action_check CHECK ((action = ANY (ARRAY['generated'::text, 'downloaded'::text])))
);

CREATE TABLE public.purchase_loi_documents (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    handoff_id uuid NOT NULL,
    attempt_id uuid NOT NULL,
    version integer NOT NULL,
    reference text NOT NULL,
    document_date date NOT NULL,
    format text NOT NULL,
    request_hash text NOT NULL,
    content jsonb NOT NULL,
    sha256 text NOT NULL,
    object_bucket text NOT NULL,
    object_key text NOT NULL,
    byte_size bigint NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_loi_documents_byte_size_check CHECK ((byte_size > 0)),
    CONSTRAINT purchase_loi_documents_content_check CHECK ((jsonb_typeof(content) = 'object'::text)),
    CONSTRAINT purchase_loi_documents_format_check CHECK ((format = ANY (ARRAY['general'::text, 'cable'::text, 'panel'::text, 'pole'::text]))),
    CONSTRAINT purchase_loi_documents_reference_check CHECK (((length(TRIM(BOTH FROM reference)) >= 1) AND (length(TRIM(BOTH FROM reference)) <= 120))),
    CONSTRAINT purchase_loi_documents_request_hash_check CHECK ((length(request_hash) = 64)),
    CONSTRAINT purchase_loi_documents_sha256_check CHECK ((length(sha256) = 64)),
    CONSTRAINT purchase_loi_documents_version_check CHECK ((version > 0))
);

CREATE TABLE public.purchase_loi_reviews (
    id uuid NOT NULL,
    document_id uuid NOT NULL,
    details_correct boolean NOT NULL,
    prices_correct boolean NOT NULL,
    format_correct boolean NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid NOT NULL,
    updated_by_id uuid NOT NULL,
    CONSTRAINT purchase_loi_reviews_details_correct_check CHECK (details_correct),
    CONSTRAINT purchase_loi_reviews_format_correct_check CHECK (format_correct),
    CONSTRAINT purchase_loi_reviews_prices_correct_check CHECK (prices_correct)
);

CREATE TABLE public.purchase_order_client_file_events (
    id uuid NOT NULL,
    file_id uuid NOT NULL,
    order_id uuid NOT NULL,
    action character varying(20) NOT NULL,
    created_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    CONSTRAINT purchase_order_client_file_events_action_check CHECK (((action)::text = ANY ((ARRAY['uploaded'::character varying, 'downloaded'::character varying])::text[])))
);

CREATE TABLE public.purchase_order_client_files (
    id uuid NOT NULL,
    order_id uuid NOT NULL,
    draft_revision integer NOT NULL,
    name character varying(255) NOT NULL,
    media_type character varying(80) NOT NULL,
    byte_size bigint NOT NULL,
    sha256 character(64) NOT NULL,
    object_bucket text NOT NULL,
    object_key text NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_order_client_files_byte_size_check CHECK (((byte_size >= 1) AND (byte_size <= 10485760))),
    CONSTRAINT purchase_order_client_files_draft_revision_check CHECK ((draft_revision > 0)),
    CONSTRAINT purchase_order_client_files_media_type_check CHECK (((media_type)::text = ANY ((ARRAY['application/pdf'::character varying, 'image/png'::character varying, 'image/jpeg'::character varying])::text[])))
);

CREATE TABLE public.purchase_order_document_events (
    id uuid NOT NULL,
    order_id uuid NOT NULL,
    order_version integer NOT NULL,
    created_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    CONSTRAINT purchase_order_document_events_order_version_check CHECK ((order_version > 0))
);

CREATE TABLE public.purchase_order_events (
    id uuid NOT NULL,
    subject_id uuid NOT NULL,
    action character varying(30) NOT NULL,
    reason text NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    CONSTRAINT purchase_order_events_action_check CHECK (((action)::text = ANY ((ARRAY['prepare'::character varying, 'release'::character varying, 'submit_client'::character varying, 'client_decision'::character varying])::text[]))),
    CONSTRAINT purchase_order_events_reason_check CHECK (((length(TRIM(BOTH FROM reason)) >= 1) AND (length(TRIM(BOTH FROM reason)) <= 2000)))
);

CREATE TABLE public.purchase_order_sequences (
    fiscal_year integer NOT NULL,
    last_value bigint NOT NULL,
    CONSTRAINT purchase_order_sequences_fiscal_year_check CHECK (((fiscal_year >= 1) AND (fiscal_year <= 9999))),
    CONSTRAINT purchase_order_sequences_last_value_check CHECK (((last_value >= 1) AND (last_value <= 999999999)))
);

CREATE TABLE public.purchase_orders (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    package_id uuid NOT NULL,
    sourcing_id uuid NOT NULL,
    technical_id uuid NOT NULL,
    version integer NOT NULL,
    state character varying(20) NOT NULL,
    number character varying(80),
    source_snapshot jsonb NOT NULL,
    data jsonb NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_orders_check CHECK (((((state)::text = 'draft'::text) AND (number IS NULL)) OR (((state)::text = 'released'::text) AND (number IS NOT NULL)))),
    CONSTRAINT purchase_orders_data_check CHECK ((jsonb_typeof(data) = 'object'::text)),
    CONSTRAINT purchase_orders_source_snapshot_check CHECK ((jsonb_typeof(source_snapshot) = 'object'::text)),
    CONSTRAINT purchase_orders_state_check CHECK (((state)::text = ANY ((ARRAY['draft'::character varying, 'released'::character varying])::text[]))),
    CONSTRAINT purchase_orders_version_check CHECK ((version > 0))
);

CREATE TABLE public.purchase_project_vendors (
    project_id uuid NOT NULL,
    vendor_ids uuid[] NOT NULL,
    version integer NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_by_id uuid NOT NULL,
    item_sets jsonb,
    CONSTRAINT purchase_project_vendors_item_sets_check CHECK ((jsonb_typeof(item_sets) = 'array'::text)),
    CONSTRAINT purchase_project_vendors_vendor_ids_check CHECK ((cardinality(vendor_ids) >= 3)),
    CONSTRAINT purchase_project_vendors_version_check CHECK ((version > 0))
);

CREATE TABLE public.purchase_quality_cases (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    package_id uuid NOT NULL,
    report_id uuid NOT NULL,
    report_version integer NOT NULL,
    version integer NOT NULL,
    revision integer NOT NULL,
    source_snapshot jsonb NOT NULL,
    data jsonb NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_quality_cases_data_check CHECK ((jsonb_typeof(data) = 'object'::text)),
    CONSTRAINT purchase_quality_cases_report_version_check CHECK ((report_version > 0)),
    CONSTRAINT purchase_quality_cases_revision_check CHECK ((revision > 0)),
    CONSTRAINT purchase_quality_cases_source_snapshot_check CHECK ((jsonb_typeof(source_snapshot) = 'object'::text)),
    CONSTRAINT purchase_quality_cases_version_check CHECK ((version > 0))
);

CREATE TABLE public.purchase_quality_certificates (
    case_id uuid NOT NULL,
    code character varying(20) NOT NULL,
    title text NOT NULL,
    cert_number character varying(255) NOT NULL,
    issued_by character varying(255) NOT NULL,
    valid_until date,
    file_name character varying(500) NOT NULL,
    verified boolean NOT NULL,
    verified_at timestamp with time zone,
    verified_by_id uuid,
    data jsonb NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_quality_certificates_check CHECK (((verified AND (verified_at IS NOT NULL) AND (verified_by_id IS NOT NULL) AND (length(TRIM(BOTH FROM cert_number)) > 0) AND (length(TRIM(BOTH FROM issued_by)) > 0)) OR ((NOT verified) AND (verified_at IS NULL) AND (verified_by_id IS NULL)))),
    CONSTRAINT purchase_quality_certificates_code_check CHECK (((code)::text = ANY ((ARRAY['TC'::character varying, 'TR'::character varying, 'MTC'::character varying, 'RTC'::character varying, 'Warranty'::character varying])::text[]))),
    CONSTRAINT purchase_quality_certificates_data_check CHECK ((jsonb_typeof(data) = 'object'::text))
);

CREATE TABLE public.purchase_quality_events (
    id uuid NOT NULL,
    subject_id uuid NOT NULL,
    action character varying(20) NOT NULL,
    certificate_code character varying(20) NOT NULL,
    reason text NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    CONSTRAINT purchase_quality_events_action_check CHECK (((action)::text = ANY ((ARRAY['record'::character varying, 'verify'::character varying, 'revoke'::character varying, 'revise'::character varying])::text[]))),
    CONSTRAINT purchase_quality_events_check CHECK (((((action)::text = 'revise'::text) AND ((certificate_code)::text = ''::text)) OR (((action)::text <> 'revise'::text) AND ((certificate_code)::text = ANY ((ARRAY['TC'::character varying, 'TR'::character varying, 'MTC'::character varying, 'RTC'::character varying, 'Warranty'::character varying])::text[]))))),
    CONSTRAINT purchase_quality_events_reason_check CHECK (((length(TRIM(BOTH FROM reason)) >= 1) AND (length(TRIM(BOTH FROM reason)) <= 2000)))
);

CREATE TABLE public.purchase_quotation_attachments (
    id uuid NOT NULL,
    document_id uuid NOT NULL,
    filename character varying(255) NOT NULL,
    content_type character varying(128) NOT NULL,
    byte_size bigint NOT NULL,
    sha256 character(64) NOT NULL,
    ciphertext bytea NOT NULL,
    is_active boolean NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_quotation_attachments_byte_size_check CHECK (((byte_size > 0) AND (byte_size <= 10485760))),
    CONSTRAINT purchase_quotation_attachments_sha256_check CHECK ((sha256 ~ '^[0-9a-f]{64}$'::text))
);

CREATE TABLE public.purchase_quotation_events (
    id uuid NOT NULL,
    bo_id uuid NOT NULL,
    action character varying(40) NOT NULL,
    reason character varying(2000) NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid
);

CREATE TABLE public.purchase_quotation_evidence_events (
    id uuid NOT NULL,
    attachment_id uuid NOT NULL,
    action character varying(40) NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    CONSTRAINT purchase_quotation_evidence_events_action_check CHECK (((action)::text = ANY ((ARRAY['uploaded'::character varying, 'downloaded'::character varying])::text[])))
);

CREATE TABLE public.purchase_quotation_revisions (
    document_id uuid NOT NULL,
    revision integer NOT NULL,
    bo_id uuid NOT NULL,
    attachment_id uuid,
    quote jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    invoice_total numeric(38,2) GENERATED ALWAYS AS ((((quote -> 'calculation'::text) ->> 'total'::text))::numeric) STORED,
    CONSTRAINT purchase_quotation_revisions_revision_check CHECK ((revision > 0))
);

CREATE TABLE public.purchase_quotation_shortlist_drafts (
    project_id uuid NOT NULL,
    bo_id uuid NOT NULL,
    group_id uuid NOT NULL,
    version integer NOT NULL,
    proposals jsonb NOT NULL,
    comment character varying(2000) DEFAULT ''::character varying NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_quotation_shortlist_drafts_proposals_check CHECK (((jsonb_typeof(proposals) = 'array'::text) AND ((jsonb_array_length(proposals) >= 1) AND (jsonb_array_length(proposals) <= 2)))),
    CONSTRAINT purchase_quotation_shortlist_drafts_version_check CHECK ((version > 0))
);

CREATE TABLE public.purchase_quotation_shortlist_history (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    bo_id uuid NOT NULL,
    group_id uuid NOT NULL,
    version integer NOT NULL,
    action text NOT NULL,
    snapshot jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    CONSTRAINT purchase_quotation_shortlist_history_action_check CHECK ((action = ANY (ARRAY['saved'::text, 'submitted'::text])))
);

CREATE TABLE public.purchase_quotation_work_tasks (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    bo_id uuid NOT NULL,
    group_id uuid NOT NULL,
    attempt_id uuid NOT NULL,
    recipient_id uuid NOT NULL,
    kind text NOT NULL,
    title character varying(300) NOT NULL,
    link character varying(400) NOT NULL,
    state text NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_quotation_work_tasks_kind_check CHECK ((kind = ANY (ARRAY['finance'::text, 'executive'::text]))),
    CONSTRAINT purchase_quotation_work_tasks_link_check CHECK (((link)::text ~~ '/dashboard?%'::text)),
    CONSTRAINT purchase_quotation_work_tasks_state_check CHECK ((state = ANY (ARRAY['pending'::text, 'waiting'::text, 'negotiate'::text, 'change_vendor'::text, 'completed'::text])))
);

CREATE TABLE public.purchase_site_receipts (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    package_id uuid NOT NULL,
    dispatch_id uuid NOT NULL,
    dispatch_version integer NOT NULL,
    version integer NOT NULL,
    revision integer NOT NULL,
    status character varying(20) NOT NULL,
    source_snapshot jsonb NOT NULL,
    data jsonb NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_site_receipts_data_check CHECK ((jsonb_typeof(data) = 'object'::text)),
    CONSTRAINT purchase_site_receipts_dispatch_version_check CHECK ((dispatch_version > 0)),
    CONSTRAINT purchase_site_receipts_revision_check CHECK ((revision > 0)),
    CONSTRAINT purchase_site_receipts_source_snapshot_check CHECK ((jsonb_typeof(source_snapshot) = 'object'::text)),
    CONSTRAINT purchase_site_receipts_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'in_transit'::character varying, 'delivered_at_site'::character varying])::text[]))),
    CONSTRAINT purchase_site_receipts_version_check CHECK ((version > 0))
);

CREATE TABLE public.purchase_site_receipts_events (
    id uuid NOT NULL,
    subject_id uuid NOT NULL,
    action character varying(20) NOT NULL,
    reason text NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    CONSTRAINT purchase_site_receipts_events_action_check CHECK (((action)::text = ANY ((ARRAY['record'::character varying, 'revise'::character varying])::text[]))),
    CONSTRAINT purchase_site_receipts_events_reason_check CHECK (((length(TRIM(BOTH FROM reason)) >= 1) AND (length(TRIM(BOTH FROM reason)) <= 2000)))
);

CREATE TABLE public.purchase_sourcing_allocations (
    case_id uuid NOT NULL,
    project_id uuid NOT NULL,
    demand_id uuid NOT NULL,
    financial_reference character varying(120) NOT NULL,
    currency character varying(3) NOT NULL,
    quantity numeric(19,4) NOT NULL,
    amount numeric(16,2) NOT NULL,
    approved_quantity numeric(19,4) NOT NULL,
    approved_amount numeric(16,2) NOT NULL,
    state character varying(20) NOT NULL,
    case_version integer NOT NULL,
    is_active boolean NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_sourcing_allocations_amount_check CHECK ((amount >= (0)::numeric)),
    CONSTRAINT purchase_sourcing_allocations_approved_amount_check CHECK ((approved_amount >= (0)::numeric)),
    CONSTRAINT purchase_sourcing_allocations_approved_quantity_check CHECK ((approved_quantity > (0)::numeric)),
    CONSTRAINT purchase_sourcing_allocations_case_version_check CHECK ((case_version > 0)),
    CONSTRAINT purchase_sourcing_allocations_check CHECK ((is_active = ((state)::text <> 'cancelled'::text))),
    CONSTRAINT purchase_sourcing_allocations_check1 CHECK ((quantity <= approved_quantity)),
    CONSTRAINT purchase_sourcing_allocations_check2 CHECK ((amount <= approved_amount)),
    CONSTRAINT purchase_sourcing_allocations_quantity_check CHECK ((quantity > (0)::numeric)),
    CONSTRAINT purchase_sourcing_allocations_state_check CHECK (((state)::text = ANY ((ARRAY['pending'::character varying, 'approved'::character varying, 'released'::character varying, 'cancelled'::character varying])::text[])))
);

CREATE TABLE public.purchase_sourcing_cases (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    package_id uuid NOT NULL,
    approver_id uuid NOT NULL,
    version integer NOT NULL,
    state character varying(20) NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    source_snapshot jsonb NOT NULL,
    data jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_sourcing_cases_data_check CHECK ((jsonb_typeof(data) = 'object'::text)),
    CONSTRAINT purchase_sourcing_cases_source_snapshot_check CHECK ((jsonb_typeof(source_snapshot) = 'object'::text)),
    CONSTRAINT purchase_sourcing_cases_state_check CHECK (((state)::text = ANY ((ARRAY['draft'::character varying, 'pending'::character varying, 'approved'::character varying, 'released'::character varying])::text[]))),
    CONSTRAINT purchase_sourcing_cases_version_check CHECK ((version > 0))
);

CREATE TABLE public.purchase_sourcing_events (
    id uuid NOT NULL,
    subject_id uuid NOT NULL,
    action character varying(30) NOT NULL,
    reason character varying(2000) NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    CONSTRAINT purchase_sourcing_events_action_check CHECK (((action)::text = ANY ((ARRAY['configure'::character varying, 'rfq'::character varying, 'dispatch'::character varying, 'no_bid'::character varying, 'quote'::character varying, 'propose'::character varying, 'decide'::character varying, 'reopen'::character varying, 'release'::character varying, 'award'::character varying])::text[]))),
    CONSTRAINT purchase_sourcing_events_reason_check CHECK ((length(btrim((reason)::text)) > 0))
);

CREATE TABLE public.purchase_technical_cases (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    package_id uuid NOT NULL,
    source_case_id uuid NOT NULL,
    version integer NOT NULL,
    revision integer NOT NULL,
    state character varying(30) NOT NULL,
    source_snapshot jsonb NOT NULL,
    data jsonb NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_technical_cases_data_check CHECK ((jsonb_typeof(data) = 'object'::text)),
    CONSTRAINT purchase_technical_cases_revision_check CHECK ((revision > 0)),
    CONSTRAINT purchase_technical_cases_source_snapshot_check CHECK ((jsonb_typeof(source_snapshot) = 'object'::text)),
    CONSTRAINT purchase_technical_cases_state_check CHECK (((state)::text = ANY ((ARRAY['draft'::character varying, 'submitted'::character varying, 'under_review'::character varying, 'approved'::character varying, 'approved_with_comments'::character varying, 'revision_required'::character varying])::text[]))),
    CONSTRAINT purchase_technical_cases_version_check CHECK ((version > 0))
);

CREATE TABLE public.purchase_technical_events (
    id uuid NOT NULL,
    subject_id uuid NOT NULL,
    action character varying(40) NOT NULL,
    reason text NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    CONSTRAINT purchase_technical_events_reason_check CHECK (((length(TRIM(BOTH FROM reason)) >= 1) AND (length(TRIM(BOTH FROM reason)) <= 2000)))
);

CREATE TABLE public.purchase_technical_file_events (
    id uuid NOT NULL,
    case_id uuid NOT NULL,
    file_id uuid NOT NULL,
    action character varying(20) NOT NULL,
    reason text NOT NULL,
    created_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    CONSTRAINT purchase_technical_file_events_action_check CHECK (((action)::text = ANY ((ARRAY['uploaded'::character varying, 'downloaded'::character varying])::text[]))),
    CONSTRAINT purchase_technical_file_events_reason_check CHECK (((length(TRIM(BOTH FROM reason)) >= 1) AND (length(TRIM(BOTH FROM reason)) <= 2000)))
);

CREATE TABLE public.purchase_technical_files (
    id uuid NOT NULL,
    case_id uuid NOT NULL,
    purpose character varying(30) NOT NULL,
    name character varying(255) NOT NULL,
    media_type character varying(40) NOT NULL,
    byte_size bigint NOT NULL,
    sha256 character(64) NOT NULL,
    ciphertext bytea,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    object_bucket text DEFAULT ''::text NOT NULL,
    object_key text DEFAULT ''::text NOT NULL,
    CONSTRAINT purchase_technical_files_byte_size_check CHECK (((byte_size >= 1) AND (byte_size <= 10485760))),
    CONSTRAINT purchase_technical_files_check CHECK ((octet_length(ciphertext) > byte_size)),
    CONSTRAINT purchase_technical_files_media_type_check CHECK (((media_type)::text = ANY ((ARRAY['application/pdf'::character varying, 'image/png'::character varying, 'image/jpeg'::character varying])::text[]))),
    CONSTRAINT purchase_technical_files_purpose_check CHECK (((purpose)::text = ANY ((ARRAY['document'::character varying, 'transmittal'::character varying, 'client_approval'::character varying, 'conditions'::character varying])::text[]))),
    CONSTRAINT purchase_technical_files_sha256_check CHECK ((sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT technical_file_storage_location CHECK ((((object_bucket = ''::text) AND (object_key = ''::text) AND (ciphertext IS NOT NULL)) OR ((object_bucket <> ''::text) AND (object_key <> ''::text) AND (ciphertext IS NULL))))
);

CREATE TABLE public.purchase_type_indent_drafts (
    project_id uuid NOT NULL,
    category_code text NOT NULL,
    template_version integer,
    version integer NOT NULL,
    terms text NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_by_id uuid NOT NULL,
    template_id uuid,
    vendor_ids uuid[] DEFAULT '{}'::uuid[] NOT NULL,
    vendor_item_sets jsonb,
    CONSTRAINT purchase_type_indent_drafts_terms_check CHECK (((length(btrim(terms)) >= 1) AND (length(btrim(terms)) <= 8000))),
    CONSTRAINT purchase_type_indent_drafts_vendor_item_sets_check CHECK ((jsonb_typeof(vendor_item_sets) = 'array'::text)),
    CONSTRAINT purchase_type_indent_drafts_version_check CHECK ((version > 0)),
    CONSTRAINT purchase_type_indent_vendor_minimum CHECK (((cardinality(vendor_ids) = 0) OR (cardinality(vendor_ids) >= 3)))
);

CREATE TABLE public.purchase_work_package_events (
    id uuid NOT NULL,
    subject_id uuid NOT NULL,
    action text NOT NULL,
    reason text NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    CONSTRAINT purchase_work_package_events_action_check CHECK ((action = ANY (ARRAY['create'::text, 'assign'::text, 'save'::text, 'start'::text, 'block'::text, 'resolve'::text]))),
    CONSTRAINT purchase_work_package_events_reason_check CHECK ((length(btrim(reason)) > 0))
);

CREATE TABLE public.purchase_work_packages (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    indent_id uuid NOT NULL,
    demand_id uuid NOT NULL,
    code text NOT NULL,
    description text NOT NULL,
    source_snapshot jsonb NOT NULL,
    steps jsonb NOT NULL,
    version integer NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT purchase_work_packages_steps_check CHECK (((jsonb_typeof(steps) = 'array'::text) AND (jsonb_array_length(steps) = 20))),
    CONSTRAINT purchase_work_packages_version_check CHECK ((version > 0))
);

CREATE TABLE public.quotation_approval_events (
    id uuid NOT NULL,
    bo_id uuid NOT NULL,
    action character varying(40) NOT NULL,
    reason character varying(2000) NOT NULL,
    before_snapshot jsonb NOT NULL,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    CONSTRAINT quotation_approval_events_action_check CHECK (((action)::text = ANY ((ARRAY['submit'::character varying, 'accounts_decision'::character varying])::text[]))),
    CONSTRAINT quotation_approval_events_reason_check CHECK ((length(btrim((reason)::text)) > 0))
);

CREATE TABLE public.quotation_approval_sets (
    bo_id uuid NOT NULL,
    project_id uuid NOT NULL,
    version integer NOT NULL,
    attempts jsonb NOT NULL,
    is_active boolean NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT quotation_approval_sets_attempts_check CHECK ((jsonb_typeof(attempts) = 'array'::text)),
    CONSTRAINT quotation_approval_sets_version_check CHECK ((version > 0))
);

CREATE TABLE public.stored_document_events (
    id uuid NOT NULL,
    document_id uuid NOT NULL,
    action character varying(50) NOT NULL,
    actor_id uuid,
    metadata_snapshot jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE public.stored_documents (
    id uuid NOT NULL,
    entity_type character varying(60) NOT NULL,
    entity_id uuid NOT NULL,
    doc_type character varying(60) NOT NULL,
    title character varying(255) NOT NULL,
    original_filename character varying(255) NOT NULL,
    content_type character varying(120) NOT NULL,
    byte_size integer NOT NULL,
    storage_bucket character varying(100) NOT NULL,
    storage_key character varying(500) NOT NULL,
    sha256_hash character varying(64) NOT NULL,
    is_encrypted boolean DEFAULT true NOT NULL,
    encryption_algorithm character varying(50) DEFAULT 'AES-256-GCM'::character varying NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by_id uuid
);

CREATE TABLE public.supplier_changes (
    id uuid NOT NULL,
    supplier_id uuid NOT NULL,
    version integer NOT NULL,
    action character varying(50) NOT NULL,
    reason character varying(2000) NOT NULL,
    snapshot jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid
);

CREATE TABLE public.supplier_document_approval_changes (
    id uuid NOT NULL,
    document_id uuid NOT NULL,
    version integer NOT NULL,
    action character varying(30) NOT NULL,
    snapshot jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid
);

CREATE TABLE public.supplier_document_approvals (
    document_id uuid NOT NULL,
    supplier_id uuid NOT NULL,
    document_version integer NOT NULL,
    sha256 character(64) NOT NULL,
    state character varying(12) NOT NULL,
    comment text DEFAULT ''::text NOT NULL,
    version integer NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT supplier_document_approvals_document_version_check CHECK ((document_version > 0)),
    CONSTRAINT supplier_document_approvals_state_check CHECK (((state)::text = ANY ((ARRAY['pending'::character varying, 'accept'::character varying, 'reject'::character varying])::text[]))),
    CONSTRAINT supplier_document_approvals_version_check CHECK ((version > 0))
);

CREATE TABLE public.supplier_document_changes (
    id uuid NOT NULL,
    document_id uuid NOT NULL,
    version integer NOT NULL,
    action character varying(50) NOT NULL,
    reason character varying(2000) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid
);

CREATE TABLE public.supplier_document_tasks (
    id uuid NOT NULL,
    document_id uuid NOT NULL,
    supplier_id uuid NOT NULL,
    user_id uuid NOT NULL,
    kind character varying(12) NOT NULL,
    outcome character varying(16) DEFAULT ''::character varying NOT NULL,
    comment text DEFAULT ''::text NOT NULL,
    completed_at timestamp with time zone,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT supplier_document_tasks_check CHECK ((((completed_at IS NULL) AND ((outcome)::text = ''::text)) OR ((completed_at IS NOT NULL) AND ((outcome)::text <> ''::text)))),
    CONSTRAINT supplier_document_tasks_kind_check CHECK (((kind)::text = ANY ((ARRAY['review'::character varying, 'correction'::character varying])::text[]))),
    CONSTRAINT supplier_document_tasks_outcome_check CHECK (((outcome)::text = ANY ((ARRAY[''::character varying, 'approved'::character varying, 'rejected'::character varying, 'replaced'::character varying, 'resubmitted'::character varying, 'archived'::character varying])::text[])))
);

CREATE TABLE public.supplier_documents (
    id uuid NOT NULL,
    supplier_id uuid NOT NULL,
    doc_type character varying(50) NOT NULL,
    title character varying(255) NOT NULL,
    original_filename character varying(255) NOT NULL,
    content_type character varying(120) NOT NULL,
    byte_size integer NOT NULL,
    encrypted_content bytea,
    sha256 character varying(64) NOT NULL,
    is_active boolean NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by_id uuid,
    source_id uuid,
    source_created_at timestamp with time zone,
    source_updated_at timestamp with time zone,
    version integer DEFAULT 1 NOT NULL,
    object_bucket text,
    object_key text,
    CONSTRAINT supplier_documents_byte_size_check CHECK (((byte_size > 0) AND (byte_size <= 15728640))),
    CONSTRAINT supplier_documents_doc_type_check CHECK (((doc_type)::text = ANY ((ARRAY['aadhar'::character varying, 'pan'::character varying, 'gst'::character varying, 'cheque'::character varying, 'other'::character varying])::text[]))),
    CONSTRAINT supplier_documents_storage CHECK ((((encrypted_content IS NOT NULL) AND (object_bucket IS NULL) AND (object_key IS NULL)) OR ((encrypted_content IS NULL) AND (length(object_bucket) > 0) AND (length(object_key) > 0) AND (object_bucket IS NOT NULL) AND (object_key IS NOT NULL)))),
    CONSTRAINT supplier_documents_version_check CHECK ((version > 0))
);

CREATE TABLE public.supplier_review_assignment_changes (
    id uuid NOT NULL,
    supplier_id uuid NOT NULL,
    version integer NOT NULL,
    reason character varying(2000) NOT NULL,
    snapshot jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    CONSTRAINT supplier_review_assignment_changes_reason_check CHECK ((length(TRIM(BOTH FROM reason)) > 0)),
    CONSTRAINT supplier_review_assignment_changes_version_check CHECK ((version > 0))
);

CREATE TABLE public.supplier_review_assignments (
    id uuid NOT NULL,
    supplier_id uuid NOT NULL,
    user_id uuid NOT NULL,
    version integer NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by_id uuid,
    CONSTRAINT supplier_review_assignments_version_check CHECK ((version > 0))
);

CREATE TABLE public.supplier_verification_history (
    id uuid NOT NULL,
    supplier_id uuid NOT NULL,
    version integer NOT NULL,
    snapshot jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid
);

CREATE TABLE public.supplier_verifications (
    supplier_id uuid NOT NULL,
    version integer NOT NULL,
    fingerprint character varying(64) NOT NULL,
    decisions jsonb NOT NULL,
    reason character varying(2000) NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by_id uuid,
    CONSTRAINT supplier_verifications_decisions_check CHECK ((jsonb_typeof(decisions) = 'array'::text)),
    CONSTRAINT supplier_verifications_version_check CHECK ((version > 0))
);

CREATE TABLE public.suppliers (
    id uuid NOT NULL,
    code character varying(32) NOT NULL,
    name character varying(255) NOT NULL,
    data jsonb NOT NULL,
    is_active boolean NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by_id uuid,
    source_id uuid,
    source_version integer,
    source_created_at timestamp with time zone,
    source_updated_at timestamp with time zone,
    CONSTRAINT suppliers_data_check CHECK ((jsonb_typeof(data) = 'object'::text)),
    CONSTRAINT suppliers_source_version_check CHECK ((source_version > 0)),
    CONSTRAINT suppliers_version_check CHECK ((version > 0))
);

CREATE TABLE public.template_fields (
    id uuid NOT NULL,
    template_id uuid NOT NULL,
    field_code character varying(32) NOT NULL,
    field_name character varying(255) NOT NULL,
    input_type text NOT NULL,
    mandatory boolean NOT NULL,
    allow_na boolean NOT NULL,
    use_in_description boolean NOT NULL,
    description_order integer NOT NULL,
    description_prefix character varying(128) NOT NULL,
    description_suffix character varying(128) NOT NULL,
    use_in_duplicate_check boolean NOT NULL,
    sort_order integer NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by_id uuid,
    CONSTRAINT template_fields_check CHECK (((use_in_description AND (description_order > 0)) OR ((NOT use_in_description) AND (description_order = 0)))),
    CONSTRAINT template_fields_description_order_check CHECK ((description_order >= 0)),
    CONSTRAINT template_fields_input_type_check CHECK ((input_type = 'dropdown'::text)),
    CONSTRAINT template_fields_sort_order_check CHECK ((sort_order > 0))
);

CREATE TABLE public.template_options (
    id uuid NOT NULL,
    field_id uuid NOT NULL,
    option_code character varying(32) NOT NULL,
    option_label character varying(255) NOT NULL,
    sort_order integer NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by_id uuid,
    CONSTRAINT template_options_sort_order_check CHECK ((sort_order > 0))
);

CREATE TABLE public.template_publications (
    template_id uuid NOT NULL,
    previous_template_id uuid,
    reason text NOT NULL,
    before_definition jsonb NOT NULL,
    after_definition jsonb NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT template_publications_reason_check CHECK (((length(TRIM(BOTH FROM reason)) >= 1) AND (length(TRIM(BOTH FROM reason)) <= 1000)))
);

CREATE TABLE public.work_calendar_changes (
    id uuid NOT NULL,
    holiday_id uuid NOT NULL,
    version integer NOT NULL,
    reason text NOT NULL,
    snapshot jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid
);

CREATE TABLE public.work_calendar_holidays (
    id uuid NOT NULL,
    name text NOT NULL,
    holiday_date date NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT work_calendar_holidays_version_check CHECK ((version > 0))
);

CREATE TABLE tender.bid_costings (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    draft_id uuid NOT NULL,
    material_cost numeric(18,2) DEFAULT 0.00 NOT NULL,
    freight_cost numeric(18,2) DEFAULT 0.00 NOT NULL,
    civil_and_erection_cost numeric(18,2) DEFAULT 0.00 NOT NULL,
    contingency_percent numeric(5,2) DEFAULT 3.00 NOT NULL,
    contingency_amount numeric(18,2) DEFAULT 0.00 NOT NULL,
    overhead_percent numeric(5,2) DEFAULT 5.00 NOT NULL,
    overhead_amount numeric(18,2) DEFAULT 0.00 NOT NULL,
    margin_percent numeric(5,2) DEFAULT 10.00 NOT NULL,
    margin_amount numeric(18,2) DEFAULT 0.00 NOT NULL,
    total_bid_value numeric(18,2) DEFAULT 0.00 NOT NULL,
    status character varying(32) DEFAULT 'draft'::character varying NOT NULL,
    submission_date date,
    portal_reference character varying(255) DEFAULT ''::character varying NOT NULL,
    award_date date,
    contract_reference character varying(255) DEFAULT ''::character varying NOT NULL,
    project_id uuid,
    approved_by_id uuid,
    approved_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    CONSTRAINT bid_costings_status_check CHECK (((status)::text = ANY ((ARRAY['draft'::character varying, 'approved'::character varying, 'submitted'::character varying, 'awarded'::character varying, 'lost'::character varying])::text[])))
);

CREATE TABLE tender.bid_items (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    draft_id uuid NOT NULL,
    item_code character varying(100) DEFAULT ''::character varying NOT NULL,
    description text NOT NULL,
    category character varying(100) DEFAULT 'General'::character varying NOT NULL,
    unit character varying(32) DEFAULT 'NOS'::character varying NOT NULL,
    quantity numeric(16,4) DEFAULT 1.0000 NOT NULL,
    approved_makes text DEFAULT ''::text NOT NULL,
    estimated_rate numeric(16,2) DEFAULT 0.00 NOT NULL,
    l1_vendor_name character varying(255) DEFAULT ''::character varying NOT NULL,
    l1_unit_rate numeric(16,2) DEFAULT 0.00 NOT NULL,
    l1_total_amount numeric(16,2) DEFAULT 0.00 NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    version integer DEFAULT 1 NOT NULL
);

CREATE TABLE tender.bidding_authorizations (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    draft_id uuid NOT NULL,
    decision character varying(32) NOT NULL,
    manager_id uuid,
    estimated_value numeric(18,2),
    emd_amount numeric(18,2),
    emd_exemption_claimed boolean DEFAULT false NOT NULL,
    tender_fee numeric(18,2),
    tender_fee_exemption_claimed boolean DEFAULT false NOT NULL,
    due_date date,
    approval_notes text DEFAULT ''::text NOT NULL,
    authorized_by_id uuid,
    authorized_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    project_id uuid,
    CONSTRAINT bidding_authorizations_decision_check CHECK (((decision)::text = ANY ((ARRAY['authorized'::character varying, 'rejected'::character varying, 'conditional'::character varying])::text[])))
);

CREATE TABLE tender.document_drafts (
    id uuid NOT NULL,
    name text NOT NULL,
    owner_id uuid NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    operation_key uuid NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT document_drafts_name_check CHECK (((length(name) >= 2) AND (length(name) <= 255)))
);

CREATE TABLE tender.draft_document_removals (
    draft_id uuid NOT NULL,
    member_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid NOT NULL
);

CREATE TABLE tender.draft_events (
    id uuid NOT NULL,
    draft_id uuid NOT NULL,
    action text NOT NULL,
    receipt_id uuid,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid
);

CREATE TABLE tender.draft_sources (
    draft_id uuid NOT NULL,
    member_id uuid NOT NULL,
    document_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid
);

CREATE TABLE tender.prebid_queries (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    draft_id uuid NOT NULL,
    category character varying(32) DEFAULT 'technical'::character varying NOT NULL,
    clause_reference character varying(255) DEFAULT ''::character varying NOT NULL,
    tender_specification text NOT NULL,
    query_description text NOT NULL,
    justification text DEFAULT ''::text NOT NULL,
    status character varying(32) DEFAULT 'draft'::character varying NOT NULL,
    client_response text DEFAULT ''::text NOT NULL,
    corrigendum_reference character varying(255) DEFAULT ''::character varying NOT NULL,
    submitted_at timestamp with time zone,
    responded_at timestamp with time zone,
    created_by_id uuid,
    updated_by_id uuid,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    CONSTRAINT prebid_queries_category_check CHECK (((category)::text = ANY ((ARRAY['technical'::character varying, 'commercial'::character varying, 'make_list'::character varying, 'boq'::character varying, 'legal'::character varying, 'other'::character varying])::text[]))),
    CONSTRAINT prebid_queries_status_check CHECK (((status)::text = ANY ((ARRAY['draft'::character varying, 'submitted'::character varying, 'answered'::character varying, 'accepted'::character varying, 'rejected'::character varying])::text[])))
);

CREATE TABLE tender.tender_capture_operations (
    actor_id uuid NOT NULL,
    command text NOT NULL,
    context_key text NOT NULL,
    idempotency_key uuid NOT NULL,
    fingerprint text NOT NULL,
    tender_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT tender_capture_operations_fingerprint_check CHECK ((fingerprint ~ '^[0-9a-f]{64}$'::text))
);

CREATE TABLE tender.tender_email_attempts (
    id uuid NOT NULL,
    job_id uuid NOT NULL,
    claim_id uuid NOT NULL,
    outcome text NOT NULL,
    error_code text,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT tender_email_attempts_outcome_check CHECK ((outcome = ANY (ARRAY['claimed'::text, 'accepted_by_provider'::text, 'failed'::text, 'outcome_unknown'::text, 'cancelled_obsolete'::text])))
);

CREATE TABLE tender.tender_email_jobs (
    id uuid NOT NULL,
    action_id uuid NOT NULL,
    recipient_id uuid NOT NULL,
    state text DEFAULT 'queued'::text NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    claim_id uuid,
    lease_until timestamp with time zone,
    next_attempt_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    error_code text,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT tender_email_jobs_attempts_check CHECK (((attempts >= 0) AND (attempts <= 5))),
    CONSTRAINT tender_email_jobs_state_check CHECK ((state = ANY (ARRAY['queued'::text, 'attempting'::text, 'accepted_by_provider'::text, 'failed'::text, 'cancelled_obsolete'::text]))),
    CONSTRAINT tender_email_jobs_version_check CHECK ((version > 0))
);

CREATE TABLE tender.tender_fms_events (
    id uuid NOT NULL,
    intake_id uuid NOT NULL,
    action character varying(40) NOT NULL,
    document_id uuid NOT NULL,
    actor_id uuid,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);

CREATE TABLE tender.tender_fms_intakes (
    id uuid NOT NULL,
    title character varying(255) NOT NULL,
    owner_id uuid NOT NULL,
    initial_sha256 character varying(64),
    version integer DEFAULT 1 NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    record_kind text DEFAULT 'legacy'::text NOT NULL,
    legacy_title text,
    sales_creator_id uuid,
    business_version bigint DEFAULT 1 NOT NULL,
    content_revision bigint DEFAULT 1 NOT NULL,
    assignment_version bigint DEFAULT 1 NOT NULL,
    manager_assignment_id uuid,
    receipt_id uuid,
    external_reference text,
    source_url text,
    CONSTRAINT tender_fms_identity_kind CHECK ((((record_kind = 'legacy'::text) AND (initial_sha256 IS NOT NULL) AND (sales_creator_id IS NULL) AND (manager_assignment_id IS NULL)) OR ((record_kind = 'named'::text) AND ((length(btrim((title)::text)) >= 2) AND (length(btrim((title)::text)) <= 255)) AND (sales_creator_id IS NOT NULL) AND (manager_assignment_id IS NOT NULL)))),
    CONSTRAINT tender_fms_intakes_assignment_version_check CHECK (((assignment_version >= 1) AND (assignment_version <= '9007199254740991'::bigint))),
    CONSTRAINT tender_fms_intakes_business_version_check CHECK (((business_version >= 1) AND (business_version <= '9007199254740991'::bigint))),
    CONSTRAINT tender_fms_intakes_content_revision_check CHECK (((content_revision >= 1) AND (content_revision <= '9007199254740991'::bigint))),
    CONSTRAINT tender_fms_intakes_external_reference_check CHECK ((length(external_reference) <= 255)),
    CONSTRAINT tender_fms_intakes_record_kind_check CHECK ((record_kind = ANY (ARRAY['legacy'::text, 'named'::text]))),
    CONSTRAINT tender_fms_intakes_source_url_check CHECK ((length(source_url) <= 2048))
);

CREATE TABLE tender.tender_fms_manager_assignments (
    id uuid NOT NULL,
    intake_id uuid NOT NULL,
    manager_id uuid NOT NULL,
    version bigint NOT NULL,
    reason text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid NOT NULL,
    CONSTRAINT tender_fms_manager_assignments_reason_check CHECK (((length(btrim(reason)) >= 10) AND (length(btrim(reason)) <= 1000))),
    CONSTRAINT tender_fms_manager_assignments_version_check CHECK (((version >= 1) AND (version <= '9007199254740991'::bigint)))
);

CREATE TABLE tender.tender_fms_owner_changes (
    id uuid NOT NULL,
    intake_id uuid NOT NULL,
    previous_owner_id uuid NOT NULL,
    owner_id uuid NOT NULL,
    version integer NOT NULL,
    reason text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid NOT NULL,
    CONSTRAINT tender_fms_owner_changes_check CHECK ((previous_owner_id <> owner_id)),
    CONSTRAINT tender_fms_owner_changes_reason_check CHECK (((length(reason) >= 10) AND (length(reason) <= 1000)))
);

CREATE TABLE tender.tender_fms_participant_grants (
    id uuid NOT NULL,
    intake_id uuid NOT NULL,
    user_id uuid NOT NULL,
    version bigint NOT NULL,
    is_active boolean NOT NULL,
    valid_until timestamp with time zone,
    reason text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid NOT NULL,
    CONSTRAINT tender_fms_participant_grants_reason_check CHECK (((length(btrim(reason)) >= 10) AND (length(btrim(reason)) <= 1000))),
    CONSTRAINT tender_fms_participant_grants_version_check CHECK (((version >= 1) AND (version <= '9007199254740991'::bigint)))
);

CREATE TABLE tender.tender_fms_record_events (
    id uuid NOT NULL,
    intake_id uuid NOT NULL,
    business_version bigint NOT NULL,
    action text NOT NULL,
    actor_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT tender_fms_record_events_action_check CHECK ((action = ANY (ARRAY['named_record_created'::text, 'metadata_updated'::text, 'manager_assigned'::text]))),
    CONSTRAINT tender_fms_record_events_business_version_check CHECK (((business_version >= 1) AND (business_version <= '9007199254740991'::bigint)))
);

CREATE TABLE tender.tender_provision_operations (
    id uuid NOT NULL,
    tender_id uuid NOT NULL,
    state text DEFAULT 'pending'::text NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    blocker_code text DEFAULT 'project_provider_pending'::text NOT NULL,
    project_id uuid,
    estimate_id uuid,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT tender_provision_operations_check CHECK (((state = 'ready'::text) = ((project_id IS NOT NULL) AND (estimate_id IS NOT NULL)))),
    CONSTRAINT tender_provision_operations_state_check CHECK ((state = ANY (ARRAY['pending'::text, 'ready'::text, 'blocked_configuration'::text, 'failed_retryable'::text, 'needs_reconciliation'::text]))),
    CONSTRAINT tender_provision_operations_version_check CHECK ((version > 0))
);

CREATE TABLE tender.tender_source_actions (
    id uuid NOT NULL,
    tender_id uuid NOT NULL,
    assignment_id uuid NOT NULL,
    recipient_id uuid NOT NULL,
    source_event_id uuid NOT NULL,
    assignment_version bigint NOT NULL,
    notice_id uuid,
    kind text DEFAULT 'manager_evaluation'::text NOT NULL,
    state text DEFAULT 'pending'::text NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    CONSTRAINT tender_source_actions_assignment_version_check CHECK ((assignment_version > 0)),
    CONSTRAINT tender_source_actions_kind_check CHECK ((kind = 'manager_evaluation'::text)),
    CONSTRAINT tender_source_actions_state_check CHECK ((state = ANY (ARRAY['pending'::text, 'cancelled_obsolete'::text, 'completed'::text])))
);

CREATE TABLE tender.tender_source_links (
    member_id uuid NOT NULL,
    tender_id uuid NOT NULL,
    document_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    created_by_id uuid
);

CREATE TABLE tender.tsc_cloud_calls (
    id uuid NOT NULL,
    draft_id uuid NOT NULL,
    job_id uuid NOT NULL,
    actor_id uuid NOT NULL,
    request_key text NOT NULL,
    fingerprint text NOT NULL,
    provider text NOT NULL,
    model text NOT NULL,
    reserved_paise bigint NOT NULL,
    state text DEFAULT 'pending'::text NOT NULL,
    error_code text DEFAULT ''::text NOT NULL,
    result_cipher bytea,
    result_hash text,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT tsc_cloud_calls_check CHECK (((result_cipher IS NULL) = (result_hash IS NULL))),
    CONSTRAINT tsc_cloud_calls_check1 CHECK (((state = 'ready'::text) = (result_cipher IS NOT NULL))),
    CONSTRAINT tsc_cloud_calls_provider_check CHECK ((provider = ANY (ARRAY['gemini'::text, 'openai'::text]))),
    CONSTRAINT tsc_cloud_calls_request_key_check CHECK (((length(request_key) >= 1) AND (length(request_key) <= 200))),
    CONSTRAINT tsc_cloud_calls_reserved_paise_check CHECK (((reserved_paise >= 1) AND (reserved_paise <= 10000))),
    CONSTRAINT tsc_cloud_calls_state_check CHECK ((state = ANY (ARRAY['pending'::text, 'ready'::text, 'failed'::text])))
);

CREATE TABLE tender.tsc_document_text (
    member_id uuid NOT NULL,
    reader_version text NOT NULL,
    source_sha256 text NOT NULL,
    payload_cipher bytea NOT NULL,
    payload_hash text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT tsc_document_text_payload_hash_check CHECK ((payload_hash ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT tsc_document_text_source_sha256_check CHECK ((source_sha256 ~ '^[0-9a-f]{64}$'::text))
);

CREATE TABLE tender.tsc_eligibility_verdicts (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    draft_id uuid NOT NULL,
    job_id uuid NOT NULL,
    verdict character varying(32) NOT NULL,
    overall_summary text NOT NULL,
    criteria_matrix jsonb DEFAULT '[]'::jsonb NOT NULL,
    gaps_and_risks jsonb DEFAULT '[]'::jsonb NOT NULL,
    model_used character varying(100) DEFAULT ''::character varying NOT NULL,
    evaluated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    human_verified boolean DEFAULT false NOT NULL,
    human_decision character varying(32) DEFAULT 'pending'::character varying NOT NULL,
    human_remarks text DEFAULT ''::text NOT NULL,
    verified_by_id uuid,
    verified_at timestamp with time zone,
    version integer DEFAULT 1 NOT NULL,
    CONSTRAINT tsc_eligibility_verdicts_human_decision_check CHECK (((human_decision)::text = ANY ((ARRAY['pending'::character varying, 'accepted'::character varying, 'rejected'::character varying, 'overridden'::character varying])::text[]))),
    CONSTRAINT tsc_eligibility_verdicts_verdict_check CHECK (((verdict)::text = ANY ((ARRAY['eligible'::character varying, 'not_eligible'::character varying, 'conditionally_eligible'::character varying])::text[])))
);

CREATE TABLE tender.tsc_events (
    id uuid NOT NULL,
    job_id uuid NOT NULL,
    action text NOT NULL,
    version bigint NOT NULL,
    reason text DEFAULT ''::text NOT NULL,
    result_cipher bytea,
    result_hash text,
    created_by_id uuid,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);

CREATE TABLE tender.tsc_format_events (
    id uuid NOT NULL,
    format_id uuid NOT NULL,
    version bigint NOT NULL,
    actor_id uuid NOT NULL,
    before_snapshot jsonb,
    after_snapshot jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);

CREATE TABLE tender.tsc_format_questions (
    format_id uuid NOT NULL,
    id text NOT NULL,
    "position" integer NOT NULL,
    section text NOT NULL,
    printed_number text NOT NULL,
    question text NOT NULL,
    question_key text GENERATED ALWAYS AS (tender.tsc_question_key(question)) STORED,
    manual_review boolean DEFAULT false NOT NULL,
    legacy_row integer DEFAULT 0 NOT NULL,
    CONSTRAINT tsc_format_questions_id_check CHECK ((id ~ '^[a-zA-Z0-9_.-]{1,80}$'::text)),
    CONSTRAINT tsc_format_questions_position_check CHECK ((("position" >= 1) AND ("position" <= 200))),
    CONSTRAINT tsc_format_questions_printed_number_check CHECK ((char_length(printed_number) <= 30)),
    CONSTRAINT tsc_format_questions_question_check CHECK (((char_length(btrim(question)) >= 3) AND (char_length(btrim(question)) <= 2000))),
    CONSTRAINT tsc_format_questions_section_check CHECK (((char_length(btrim(section)) >= 1) AND (char_length(btrim(section)) <= 80)))
);

CREATE TABLE tender.tsc_formats (
    id uuid NOT NULL,
    name text NOT NULL,
    name_key text GENERATED ALWAYS AS (tender.tsc_question_key(name)) STORED,
    description text DEFAULT ''::text NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    created_by_id uuid,
    updated_by_id uuid,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    purpose text DEFAULT 'Generic'::text NOT NULL,
    CONSTRAINT tsc_formats_description_check CHECK ((char_length(description) <= 1000)),
    CONSTRAINT tsc_formats_name_check CHECK (((char_length(btrim(name)) >= 3) AND (char_length(btrim(name)) <= 120))),
    CONSTRAINT tsc_formats_purpose_check CHECK (((char_length(btrim(purpose)) >= 1) AND (char_length(btrim(purpose)) <= 80))),
    CONSTRAINT tsc_formats_version_check CHECK ((version > 0))
);

CREATE TABLE tender.tsc_jobs (
    id uuid NOT NULL,
    draft_id uuid NOT NULL,
    draft_version bigint NOT NULL,
    actor_id uuid NOT NULL,
    operation_key uuid NOT NULL,
    mode text NOT NULL,
    model text NOT NULL,
    state text DEFAULT 'queued'::text NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    claim_id uuid,
    lease_until timestamp with time zone,
    error_code text DEFAULT ''::text NOT NULL,
    input_cipher bytea NOT NULL,
    input_hash text NOT NULL,
    result_cipher bytea,
    result_hash text,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    format_id uuid,
    format_version bigint,
    CONSTRAINT tsc_job_format_pair CHECK ((((format_id IS NULL) = (format_version IS NULL)) AND ((format_version IS NULL) OR (format_version > 0)))),
    CONSTRAINT tsc_jobs_attempts_check CHECK (((attempts >= 0) AND (attempts <= 3))),
    CONSTRAINT tsc_jobs_check CHECK (((state = 'processing'::text) = ((claim_id IS NOT NULL) AND (lease_until IS NOT NULL)))),
    CONSTRAINT tsc_jobs_check1 CHECK (((result_cipher IS NULL) = (result_hash IS NULL))),
    CONSTRAINT tsc_jobs_draft_version_check CHECK ((draft_version > 0)),
    CONSTRAINT tsc_jobs_mode_check CHECK ((mode = ANY (ARRAY['manual'::text, 'local'::text, 'ollama'::text, 'gemini'::text, 'openai'::text]))),
    CONSTRAINT tsc_jobs_state_check CHECK ((state = ANY (ARRAY['queued'::text, 'processing'::text, 'ready'::text, 'failed'::text])))
);

CREATE TABLE tender.tsc_model_calls (
    id uuid NOT NULL,
    job_id uuid NOT NULL,
    draft_id uuid NOT NULL,
    actor_id uuid NOT NULL,
    task_id uuid NOT NULL,
    request_key text NOT NULL,
    provider text NOT NULL,
    model text NOT NULL,
    purpose text NOT NULL,
    manifest jsonb NOT NULL,
    pricing jsonb NOT NULL,
    reserved_paise bigint NOT NULL,
    state text DEFAULT 'reserved'::text NOT NULL,
    error_code text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT tsc_model_calls_provider_check CHECK ((provider = ANY (ARRAY['gemini'::text, 'openai'::text, 'deepseek'::text, 'groq'::text]))),
    CONSTRAINT tsc_model_calls_reserved_paise_check CHECK (((reserved_paise >= 1) AND (reserved_paise <= 10000))),
    CONSTRAINT tsc_model_calls_state_check CHECK ((state = ANY (ARRAY['reserved'::text, 'sent'::text, 'completed'::text, 'failed'::text, 'uncertain'::text])))
);

CREATE TABLE tender.tsc_model_responses (
    id uuid NOT NULL,
    call_id uuid NOT NULL,
    job_id uuid NOT NULL,
    draft_id uuid NOT NULL,
    http_status integer NOT NULL,
    provider_request_id text DEFAULT ''::text NOT NULL,
    raw_cipher bytea NOT NULL,
    raw_hash text NOT NULL,
    parsed_cipher bytea NOT NULL,
    parsed_hash text NOT NULL,
    usage jsonb NOT NULL,
    estimated_micro_inr bigint,
    cost_basis text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    input_evidence jsonb DEFAULT '{}'::jsonb NOT NULL,
    CONSTRAINT tsc_model_responses_cost_basis_check CHECK ((cost_basis = ANY (ARRAY['reported_usage'::text, 'free_tier'::text, 'usage_unavailable'::text]))),
    CONSTRAINT tsc_model_responses_estimated_micro_inr_check CHECK ((estimated_micro_inr >= 0))
);

CREATE TABLE tender.tsc_workspace_events (
    id uuid NOT NULL,
    job_id uuid NOT NULL,
    version bigint NOT NULL,
    action text NOT NULL,
    reason text DEFAULT ''::text NOT NULL,
    payload_cipher bytea NOT NULL,
    payload_hash text NOT NULL,
    created_by_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);

CREATE TABLE tender.tsc_workspace_tasks (
    id uuid NOT NULL,
    job_id uuid NOT NULL,
    draft_id uuid NOT NULL,
    actor_id uuid NOT NULL,
    operation_key uuid NOT NULL,
    request_hash text NOT NULL,
    question_id text,
    provider text NOT NULL,
    stronger boolean DEFAULT false NOT NULL,
    state text DEFAULT 'queued'::text NOT NULL,
    claim_id uuid,
    lease_until timestamp with time zone,
    error_code text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT tsc_workspace_tasks_check CHECK (((state = 'processing'::text) = ((claim_id IS NOT NULL) AND (lease_until IS NOT NULL)))),
    CONSTRAINT tsc_workspace_tasks_provider_check CHECK ((provider = ANY (ARRAY['gemini'::text, 'openai'::text, 'deepseek'::text, 'groq'::text]))),
    CONSTRAINT tsc_workspace_tasks_state_check CHECK ((state = ANY (ARRAY['queued'::text, 'processing'::text, 'ready'::text, 'failed'::text])))
);

CREATE TABLE tender.tsc_workspaces (
    id uuid NOT NULL,
    draft_id uuid NOT NULL,
    draft_version bigint NOT NULL,
    actor_id uuid NOT NULL,
    operation_key uuid NOT NULL,
    request_hash text NOT NULL,
    mode text NOT NULL,
    state text NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    snapshot_cipher bytea NOT NULL,
    snapshot_hash text NOT NULL,
    answers_cipher bytea NOT NULL,
    answers_hash text NOT NULL,
    claim_id uuid,
    lease_until timestamp with time zone,
    error_code text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT tsc_workspaces_check CHECK (((state = 'processing'::text) = ((claim_id IS NOT NULL) AND (lease_until IS NOT NULL)))),
    CONSTRAINT tsc_workspaces_mode_check CHECK ((mode = ANY (ARRAY['manual'::text, 'gemini'::text, 'openai'::text, 'deepseek'::text, 'groq'::text]))),
    CONSTRAINT tsc_workspaces_state_check CHECK ((state = ANY (ARRAY['queued'::text, 'processing'::text, 'ready'::text, 'failed'::text]))),
    CONSTRAINT tsc_workspaces_version_check CHECK ((version > 0))
);

CREATE TABLE tender.vendor_quotations (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    draft_id uuid NOT NULL,
    bid_item_id uuid NOT NULL,
    vendor_id uuid,
    vendor_name character varying(255) NOT NULL,
    make_offered character varying(255) DEFAULT ''::character varying NOT NULL,
    unit_rate numeric(16,2) DEFAULT 0.00 NOT NULL,
    freight_and_insurance numeric(16,2) DEFAULT 0.00 NOT NULL,
    gst_percent numeric(5,2) DEFAULT 18.00 NOT NULL,
    delivery_weeks integer DEFAULT 4 NOT NULL,
    is_technically_compliant boolean DEFAULT true NOT NULL,
    is_l1 boolean DEFAULT false NOT NULL,
    remarks text DEFAULT ''::text NOT NULL,
    quote_ref character varying(100) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    version integer DEFAULT 1 NOT NULL
);

ALTER TABLE ONLY public.activity_events ATTACH PARTITION public.activity_events_project FOR VALUES IN ('project');

ALTER TABLE ONLY public.activity_events ATTACH PARTITION public.activity_events_tender FOR VALUES IN ('tender');

INSERT INTO public.application_purchase_settings (singleton, item_list_mode, updated_at, updated_by_id) VALUES
	(true, 'single', DEFAULT, NULL);

INSERT INTO public.epcerp_schema_identity (singleton, project) VALUES
	(true, 'epcERP');

INSERT INTO public.purchase_indent_templates (category_code, version, terms, created_at, created_by_id, template_id, name) VALUES
	('CAB', 1, 'Scope: Quote each cable and accessory separately against the BOQ. State whether laying, termination and testing are included.
Approval documents: Submit cable GTP/TDS, QAP and required test certificates.
Inspection and dispatch: Confirm inspection authority, notice period and third-party inspection cost. Provide acceptance and routine test reports, BIS evidence and QAP test records. Dispatch only after clearance.
Warranty: Confirm onsite coverage and the applicable commissioning and supply periods.
Commercial terms: Confirm delivery site and price basis, freight, taxes, lead time, payment terms, quote validity and deviations.', DEFAULT, NULL, '00000000-0000-0000-0000-000000000001', 'Standard'),
	('PAN', 1, 'Scope: Confirm enclosure, incomer/outgoing ratings, busbar and controls against the specification. State any installation or foundation work included and quote approved makes.
Approval documents: Provide GA drawing, SLD, control wiring diagram, BOM and QAP.
Inspection and dispatch: Confirm inspection authority, notice period and cost. Provide test, calibration and warranty certificates. Dispatch only after clearance.
Warranty: Confirm onsite coverage and the applicable commissioning and supply periods.
Commercial terms: Confirm delivery site and price basis, freight, taxes, lead time, payment terms, quote validity and deviations.', DEFAULT, NULL, '00000000-0000-0000-0000-000000000001', 'Standard'),
	('LED', 1, 'Scope: Quote luminaire and driver as specified. Confirm wattage, efficacy, IP rating, surge protection and optical performance.
Approval documents: Provide luminaire and driver TDS and BIS evidence, accredited LM-79 and LM-80/TM-21 reports, photobiological safety evidence, applicable test certificates and QAP.
Inspection and dispatch: Confirm inspection authority, notice period and cost. Dispatch only after clearance.
Warranty: Confirm five-year onsite warranty from commissioning or five years and six months from supply, whichever is later, subject to project requirements.
Commercial terms: Confirm delivery site and price basis, freight, taxes, lead time, payment terms, quote validity and deviations.', DEFAULT, NULL, '00000000-0000-0000-0000-000000000001', 'Standard');

INSERT INTO tender.tsc_format_questions (format_id, id, "position", section, printed_number, question, manual_review, legacy_row) VALUES
	('00000000-0000-4000-8000-000000000097', 'particulars.q01', 1, 'particulars', '1', 'Name of Department:', false, 2),
	('00000000-0000-4000-8000-000000000097', 'particulars.q02', 2, 'particulars', '2', 'Name of work:', false, 3),
	('00000000-0000-4000-8000-000000000097', 'particulars.q03', 3, 'particulars', '3', 'Location where the work will be executed:', false, 4),
	('00000000-0000-4000-8000-000000000097', 'particulars.q04', 4, 'particulars', '4', 'Tender value with GST:', false, 5),
	('00000000-0000-4000-8000-000000000097', 'particulars.q05', 5, 'particulars', '5', 'Tender value without GST:', false, 6),
	('00000000-0000-4000-8000-000000000097', 'particulars.q06', 6, 'particulars', '6', 'Reverse Auction (RA) applicability :', false, 7),
	('00000000-0000-4000-8000-000000000097', 'particulars.q07', 7, 'particulars', '7', 'Completion Period:', false, 8),
	('00000000-0000-4000-8000-000000000097', 'particulars.q08', 8, 'particulars', '8', 'Pre-bid Due Date & time:', false, 9),
	('00000000-0000-4000-8000-000000000097', 'particulars.q09', 9, 'particulars', '9', 'Tender Due Date & time:', false, 10),
	('00000000-0000-4000-8000-000000000097', 'particulars.q10', 10, 'particulars', '10', 'E-tendering Portal:', false, 11),
	('00000000-0000-4000-8000-000000000097', 'particulars.q11', 11, 'particulars', '11', 'Site visit is Mandatory for the Subject tender?', false, 12),
	('00000000-0000-4000-8000-000000000097', 'particulars.q12', 12, 'particulars', '12', 'If yes provide contact person name & no & due date?', false, 13),
	('00000000-0000-4000-8000-000000000097', 'particulars.q13', 13, 'particulars', '13', 'Hard copy/Physical/Offline is required in the subject tender?', false, 14),
	('00000000-0000-4000-8000-000000000097', 'particulars.q14', 14, 'particulars', '14', 'Validity of offer', false, 15),
	('00000000-0000-4000-8000-000000000097', 'particulars.q15', 15, 'particulars', '15', 'Is Joint Venture (JV), Consortium, or MOU-based bidding allowed under this tender?', false, 16),
	('00000000-0000-4000-8000-000000000097', 'particulars.q16', 16, 'particulars', '15', 'Tender Type: Works/Goods/Services', false, 17),
	('00000000-0000-4000-8000-000000000097', 'tender_fee.q01', 17, 'tender_fee', '1', 'Is Tender Cost/ Tender Fee exemption allowed in the tender for works/ Service category? If yes', false, 21),
	('00000000-0000-4000-8000-000000000097', 'tender_fee.q02', 18, 'tender_fee', '1.1', 'Tender fee — Exemption from Startup', false, 22),
	('00000000-0000-4000-8000-000000000097', 'tender_fee.q03', 19, 'tender_fee', '1.2', 'Tender fee — Exemption from MSME (Udyam Certificate)', false, 23),
	('00000000-0000-4000-8000-000000000097', 'tender_fee.q04', 20, 'tender_fee', '1.3', 'Tender fee — Exemption against Bid Security Declaration', false, 24),
	('00000000-0000-4000-8000-000000000097', 'tender_fee.q05', 21, 'tender_fee', '2', 'If not, what is the value of tender cost and available mode of Payments?', false, 25),
	('00000000-0000-4000-8000-000000000097', 'tender_fee.q06', 22, 'tender_fee', '3', 'Is Tender Cost/ Tender is acceptable in the form of Insurance Surety Bond?', false, 26),
	('00000000-0000-4000-8000-000000000097', 'emd.q01', 23, 'emd', '1', 'Is EMD/BSD exemption allowed in the tender for work service Category?', false, 29),
	('00000000-0000-4000-8000-000000000097', 'emd.q02', 24, 'emd', '1.1', 'EMD — Exemption from Startup', false, 30),
	('00000000-0000-4000-8000-000000000097', 'emd.q03', 25, 'emd', '1.2', 'EMD — Exemption from MSME (Udyam Certificate)', false, 31),
	('00000000-0000-4000-8000-000000000097', 'emd.q04', 26, 'emd', '1.3', 'EMD — Exemption against Bid Security Declaration', false, 32),
	('00000000-0000-4000-8000-000000000097', 'emd.q05', 27, 'emd', '2', 'If not, what is the value of EMD/BSD and available mode of payments??', false, 33),
	('00000000-0000-4000-8000-000000000097', 'emd.q06', 28, 'emd', '3', 'Is EMD/BSD is acceptable in the form of Insurance Surety Bond?', false, 34),
	('00000000-0000-4000-8000-000000000097', 'similar_work.q01', 29, 'similar_work', '1', 'Matching of monetary limit of one single work, two works or three works & in how many past years?', false, 37),
	('00000000-0000-4000-8000-000000000097', 'similar_work.q02', 30, 'similar_work', '2', 'Is there any provision/clause in the tender for enhancement of the executed work value to the current price level, such as the 7% annual escalation (enhancement) clause?', false, 38),
	('00000000-0000-4000-8000-000000000097', 'similar_work.q03', 31, 'similar_work', '3', 'Definition of similar work as mentioned in the tender documents?', false, 39),
	('00000000-0000-4000-8000-000000000097', 'similar_work.q04', 32, 'similar_work', '4', 'Documents required for similar work experience?', false, 40),
	('00000000-0000-4000-8000-000000000097', 'similar_work.q05', 33, 'similar_work', '5', 'Please confirm whether Joint Venture (JV), Consortium, or MOU-based bidding is permitted under this tender. / If yes, kindly provide the detailed eligibility criteria and list of documents required to be submitted along with the bid.', false, 41),
	('00000000-0000-4000-8000-000000000097', 'similar_work.q06', 34, 'similar_work', '6', 'Any authorization from OEM required?', false, 42),
	('00000000-0000-4000-8000-000000000097', 'similar_work.q07', 35, 'similar_work', '7', 'Details of any other document if required from OEM?', false, 43),
	('00000000-0000-4000-8000-000000000097', 'similar_work.q08', 36, 'similar_work', '8', 'Details of any documents if required from client against experience submitted?', false, 44),
	('00000000-0000-4000-8000-000000000097', 'financial.q01', 37, 'financial', '1', 'Matching value of turn-over requirement?', false, 47),
	('00000000-0000-4000-8000-000000000097', 'financial.q02', 38, 'financial', '2', 'Average Annual Turnover Requirement?', false, 48),
	('00000000-0000-4000-8000-000000000097', 'financial.q03', 39, 'financial', '3', 'Documents required for Annual Turnover?', false, 49),
	('00000000-0000-4000-8000-000000000097', 'financial.q04', 40, 'financial', '4', 'If required, Any type of Certificate from Bank?', false, 50),
	('00000000-0000-4000-8000-000000000097', 'financial.q05', 41, 'financial', '5', 'If required, Any type of Certificate from CA?', false, 51),
	('00000000-0000-4000-8000-000000000097', 'financial.q06', 42, 'financial', '6', 'If required, any other type of document from any other third party or person?', false, 52),
	('00000000-0000-4000-8000-000000000097', 'financial.q07', 43, 'financial', '7', 'Solvency requirement with value?', false, 53),
	('00000000-0000-4000-8000-000000000097', 'financial.q08', 44, 'financial', '8', 'Net Worth requirement with value?', false, 54),
	('00000000-0000-4000-8000-000000000097', 'financial.q09', 45, 'financial', '9', 'Working Capital requirement with value?', false, 55),
	('00000000-0000-4000-8000-000000000097', 'electrical_licence.q01', 46, 'electrical_licence', '1', 'Is submission of an Electrical License required for this bid? If yes, please specify the state from which it should be issued?', false, 58),
	('00000000-0000-4000-8000-000000000097', 'electrical_licence.q02', 47, 'electrical_licence', '2', 'Is the required voltage level for the Electrical License specified in the tender?', false, 59),
	('00000000-0000-4000-8000-000000000097', 'electrical_licence.q03', 48, 'electrical_licence', '3', 's an Undertaking or Endorsement for the Electrical License required?', false, 60),
	('00000000-0000-4000-8000-000000000097', 'security.q01', 49, 'security', '1', 'Is the Security Deposit (SD) / Performance Bank Guarantee (PBG) accepted in the form of Insurance Surety Bond (ISB)?', false, 63),
	('00000000-0000-4000-8000-000000000097', 'security.q02', 50, 'security', '2', 'What is the percentage of Security Deposit (SD) / Performance Bank Guarantee is required?', false, 64),
	('00000000-0000-4000-8000-000000000097', 'security.q03', 51, 'security', '3', 'What is the time period of defect liability period?', false, 65),
	('00000000-0000-4000-8000-000000000097', 'security.q04', 52, 'security', '4', 'What is the time period of Warranty of LED Lights?', false, 66),
	('00000000-0000-4000-8000-000000000097', 'security.q05', 53, 'security', '5', 'Kindly confirm the time period for the Bank Guarantee applicable to SD/PBG?', false, 67),
	('00000000-0000-4000-8000-000000000097', 'payment.q01', 54, 'payment', '1', 'What are the payment terms & milestones?', false, 70),
	('00000000-0000-4000-8000-000000000097', 'payment.q02', 55, 'payment', '2', 'Who is paying the payment in the tender?', false, 71),
	('00000000-0000-4000-8000-000000000097', 'payment.q03', 56, 'payment', '3', 'Any special remark or observation for the payment?', false, 72),
	('00000000-0000-4000-8000-000000000097', 'work_location.q01', 57, 'work_location', '1', 'Confirm for stability & safety of work location? / Stability in work location can be confirmed through online news available about the location / Secondly, from vendor''s or our team member''s if any located from that area or nearby areas / Previous work experience about the place, if any / As a last resort, if required directly from vendor.', true, 87),
	('00000000-0000-4000-8000-000000000097', 'risk.q01', 58, 'risk', '1', 'Special comments on payment regularity and on time payments.', true, 90),
	('00000000-0000-4000-8000-000000000097', 'risk.q02', 59, 'risk', '2', 'Availability of local labor or Any labor union.', true, 91),
	('00000000-0000-4000-8000-000000000097', 'risk.q03', 60, 'risk', '3', 'Risk in terms of vendors/ location/ service or product to be delivered.', true, 92),
	('00000000-0000-4000-8000-000000000097', 'risk.q04', 61, 'risk', '4.', 'Is there any price escalation clause available in the tender ?', false, 93),
	('00000000-0000-4000-8000-000000000097', 'risk.q05', 62, 'risk', '4.1', 'Price escalation — if yes, what is the clause?', false, 94),
	('00000000-0000-4000-8000-000000000097', 'risk.q06', 63, 'risk', '5.', 'LD (Liquid Damage) / Penalty clause available in the tender?', false, 95),
	('00000000-0000-4000-8000-000000000097', 'risk.q07', 64, 'risk', '5.1', 'Liquidated damages / penalty — if yes, what is the clause?', false, 96),
	('00000000-0000-4000-8000-000000000097', 'conclusion.q01', 65, 'conclusion', '1', 'Are we Eligible?', true, 99),
	('00000000-0000-4000-8000-000000000097', 'conclusion.q02', 66, 'conclusion', '2', 'Should we Bid this Project?', true, 100),
	('00000000-0000-4000-8000-000000000097', 'other.q01', 67, 'other', '', 'Apart from this, are there any other conditions or documents that we need to submit in order to be technically qualified?', false, 102),
	('00000000-0000-4000-8000-000000000097', 'other.q02', 68, 'other', '', 'Are there any terms and conditions that are directly related to the pre-qualification criteria?', false, 103),
	('00000000-0000-4000-8000-000000000097', 'other.q03', 69, 'other', '', 'Kindly confirm the list of forms and formats that are required to be submitted along with the technical bid.', false, 104),
	('00000000-0000-4000-8000-000000000097', 'other.q04', 70, 'other', '', 'Any other terms, conditions, or special remarks that may affect the price bid or have price implications?', false, 105);

INSERT INTO tender.tsc_formats (id, name, description, is_active, version, created_by_id, updated_by_id, created_at, updated_at, purpose) VALUES
	('00000000-0000-4000-8000-000000000097', 'EPC Works — Standard TSC', 'Original uploaded 70-question checklist. Repeated short prompts include their fee, EMD or risk context.', true, 1, NULL, NULL, DEFAULT, DEFAULT, 'Generic');

SELECT pg_catalog.setval('auth.administration_changes_id_seq', 1, false);

SELECT pg_catalog.setval('public.project_number_sequence', 1, false);

ALTER TABLE ONLY auth.access_sessions
    ADD CONSTRAINT access_sessions_pkey PRIMARY KEY (token_hash);

ALTER TABLE ONLY auth.administration_changes
    ADD CONSTRAINT administration_changes_pkey PRIMARY KEY (id);

ALTER TABLE ONLY auth.auth_bootstrap_identity
    ADD CONSTRAINT auth_bootstrap_identity_pkey PRIMARY KEY (singleton);

ALTER TABLE ONLY auth.auth_bootstrap_identity
    ADD CONSTRAINT auth_bootstrap_identity_user_id_key UNIQUE (user_id);

ALTER TABLE ONLY auth.auth_credential_changes
    ADD CONSTRAINT auth_credential_changes_pkey PRIMARY KEY (id);

ALTER TABLE ONLY auth.auth_credential_emails
    ADD CONSTRAINT auth_credential_emails_pkey PRIMARY KEY (id);

ALTER TABLE ONLY auth.auth_departments
    ADD CONSTRAINT auth_departments_code_key UNIQUE (code);

ALTER TABLE ONLY auth.auth_departments
    ADD CONSTRAINT auth_departments_pkey PRIMARY KEY (id);

ALTER TABLE ONLY auth.auth_offboarding_cases
    ADD CONSTRAINT auth_offboarding_cases_pkey PRIMARY KEY (id);

ALTER TABLE ONLY auth.auth_offboarding_changes
    ADD CONSTRAINT auth_offboarding_changes_case_id_version_key UNIQUE (case_id, version);

ALTER TABLE ONLY auth.auth_offboarding_changes
    ADD CONSTRAINT auth_offboarding_changes_pkey PRIMARY KEY (id);

ALTER TABLE ONLY auth.auth_organisation_changes
    ADD CONSTRAINT auth_organisation_changes_entity_id_entity_type_version_key UNIQUE (entity_id, entity_type, version);

ALTER TABLE ONLY auth.auth_organisation_changes
    ADD CONSTRAINT auth_organisation_changes_pkey PRIMARY KEY (id);

ALTER TABLE ONLY auth.auth_password_recovery
    ADD CONSTRAINT auth_password_recovery_pkey PRIMARY KEY (token_hash);

ALTER TABLE ONLY auth.auth_personal_contact_changes
    ADD CONSTRAINT auth_personal_contact_changes_pkey PRIMARY KEY (id);

ALTER TABLE ONLY auth.auth_personal_contact_changes
    ADD CONSTRAINT auth_personal_contact_changes_user_id_version_key UNIQUE (user_id, version);

ALTER TABLE ONLY auth.auth_reporting_lines
    ADD CONSTRAINT auth_reporting_lines_pkey PRIMARY KEY (user_id);

ALTER TABLE ONLY auth.auth_user_status_changes
    ADD CONSTRAINT auth_user_status_changes_pkey PRIMARY KEY (id);

ALTER TABLE ONLY auth.auth_user_status_changes
    ADD CONSTRAINT auth_user_status_changes_user_id_credential_version_key UNIQUE (user_id, credential_version);

ALTER TABLE ONLY auth.auth_username_changes
    ADD CONSTRAINT auth_username_changes_pkey PRIMARY KEY (id);

ALTER TABLE ONLY auth.auth_username_changes
    ADD CONSTRAINT auth_username_changes_user_id_version_key UNIQUE (user_id, version);

ALTER TABLE ONLY auth.email_changes
    ADD CONSTRAINT email_changes_pkey PRIMARY KEY (token_hash);

ALTER TABLE ONLY auth.login_attempts
    ADD CONSTRAINT login_attempts_pkey PRIMARY KEY (identifier_hash);

ALTER TABLE ONLY auth.permissions
    ADD CONSTRAINT permissions_code_key UNIQUE (code);

ALTER TABLE ONLY auth.permissions
    ADD CONSTRAINT permissions_pkey PRIMARY KEY (id);

ALTER TABLE ONLY auth.refresh_tokens
    ADD CONSTRAINT refresh_tokens_pkey PRIMARY KEY (id);

ALTER TABLE ONLY auth.role_permissions
    ADD CONSTRAINT role_permissions_pkey PRIMARY KEY (role_id, permission_id);

ALTER TABLE ONLY auth.roles
    ADD CONSTRAINT roles_name_key UNIQUE (name);

ALTER TABLE ONLY auth.roles
    ADD CONSTRAINT roles_pkey PRIMARY KEY (id);

ALTER TABLE ONLY auth.refresh_tokens
    ADD CONSTRAINT uq_refresh_tokens_token_hash UNIQUE (token_hash);

ALTER TABLE ONLY auth.user_activity
    ADD CONSTRAINT user_activity_pkey PRIMARY KEY (user_id);

ALTER TABLE ONLY auth.user_profile
    ADD CONSTRAINT user_profiles_pkey PRIMARY KEY (user_id);

ALTER TABLE ONLY auth.user_roles
    ADD CONSTRAINT user_roles_pkey PRIMARY KEY (user_id, role_id);

ALTER TABLE ONLY auth.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.activity_events
    ADD CONSTRAINT activity_events_pkey PRIMARY KEY (scope_kind, scope_id, id, action);

ALTER TABLE ONLY public.activity_events_project
    ADD CONSTRAINT activity_events_project_pkey PRIMARY KEY (scope_kind, scope_id, id, action);

ALTER TABLE ONLY public.activity_events_tender
    ADD CONSTRAINT activity_events_tender_pkey PRIMARY KEY (scope_kind, scope_id, id, action);

ALTER TABLE ONLY public.application_purchase_settings
    ADD CONSTRAINT application_purchase_settings_pkey PRIMARY KEY (singleton);

ALTER TABLE ONLY public.business_events
    ADD CONSTRAINT business_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.business_events
    ADD CONSTRAINT business_events_source_module_entity_type_entity_id_entity__key UNIQUE (source_module, entity_type, entity_id, entity_version, action);

ALTER TABLE ONLY public.catalogue_item_events
    ADD CONSTRAINT catalogue_item_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.catalogue_items
    ADD CONSTRAINT catalogue_items_fingerprint_key UNIQUE (fingerprint);

ALTER TABLE ONLY public.catalogue_items
    ADD CONSTRAINT catalogue_items_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.catalogue_items
    ADD CONSTRAINT catalogue_items_sku_key UNIQUE (sku);

ALTER TABLE ONLY public.catalogue_units
    ADD CONSTRAINT catalogue_units_code_key UNIQUE (code);

ALTER TABLE ONLY public.catalogue_units
    ADD CONSTRAINT catalogue_units_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.categories
    ADD CONSTRAINT categories_code_key UNIQUE (code);

ALTER TABLE ONLY public.categories
    ADD CONSTRAINT categories_id_unit_unique UNIQUE (id, unit_id);

ALTER TABLE ONLY public.categories
    ADD CONSTRAINT categories_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.category_catalogue_changes
    ADD CONSTRAINT category_catalogue_changes_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.category_images
    ADD CONSTRAINT category_images_pkey PRIMARY KEY (category_id);

ALTER TABLE ONLY public.category_subcategories
    ADD CONSTRAINT category_subcategories_code_key UNIQUE (code);

ALTER TABLE ONLY public.category_subcategories
    ADD CONSTRAINT category_subcategories_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.client_contacts
    ADD CONSTRAINT client_contacts_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.client_purchase_rule_events
    ADD CONSTRAINT client_purchase_rule_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.client_purchase_rules
    ADD CONSTRAINT client_purchase_rules_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.client_types
    ADD CONSTRAINT client_types_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.clients
    ADD CONSTRAINT clients_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.communication_notice_events
    ADD CONSTRAINT communication_notice_events_notice_id_notice_version_key UNIQUE (notice_id, notice_version);

ALTER TABLE ONLY public.communication_notice_events
    ADD CONSTRAINT communication_notice_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.communication_notices
    ADD CONSTRAINT communication_notices_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.communication_notices
    ADD CONSTRAINT communication_source_recipient UNIQUE (source_event_id, recipient_id);

ALTER TABLE ONLY public.company_certificates
    ADD CONSTRAINT company_certificates_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.company_financial_years
    ADD CONSTRAINT company_financial_years_financial_year_key UNIQUE (financial_year);

ALTER TABLE ONLY public.company_financial_years
    ADD CONSTRAINT company_financial_years_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.company_licences
    ADD CONSTRAINT company_licences_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.company_licences
    ADD CONSTRAINT company_licences_state_licence_number_key UNIQUE (state, licence_number);

ALTER TABLE ONLY public.company_profile
    ADD CONSTRAINT company_profile_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.company_profile
    ADD CONSTRAINT company_profile_singleton_key UNIQUE (singleton);

ALTER TABLE ONLY public.company_work_experience
    ADD CONSTRAINT company_work_experience_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.configuration_templates
    ADD CONSTRAINT configuration_templates_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.configuration_templates
    ADD CONSTRAINT configuration_templates_subcategory_id_revision_key UNIQUE (subcategory_id, revision);

ALTER TABLE ONLY public.document_package_events
    ADD CONSTRAINT document_package_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.document_package_files
    ADD CONSTRAINT document_package_files_object_key_key UNIQUE (object_key);

ALTER TABLE ONLY public.document_package_files
    ADD CONSTRAINT document_package_files_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.document_package_files
    ADD CONSTRAINT document_package_files_receipt_id_client_file_key_key UNIQUE (receipt_id, client_file_key);

ALTER TABLE ONLY public.document_package_files
    ADD CONSTRAINT document_package_files_receipt_id_id_key UNIQUE (receipt_id, id);

ALTER TABLE ONLY public.document_package_manifest
    ADD CONSTRAINT document_package_manifest_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.document_package_members
    ADD CONSTRAINT document_package_members_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.document_package_members
    ADD CONSTRAINT document_package_members_receipt_id_validation_claim_id_nor_key UNIQUE (receipt_id, validation_claim_id, normalized_path);

ALTER TABLE ONLY public.document_package_objects
    ADD CONSTRAINT document_package_objects_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.document_package_operations
    ADD CONSTRAINT document_package_operations_pkey PRIMARY KEY (actor_id, command, context_key, idempotency_key);

ALTER TABLE ONLY public.document_package_receipts
    ADD CONSTRAINT document_package_receipts_consumed_by_tender_id_key UNIQUE (consumed_by_tender_id);

ALTER TABLE ONLY public.document_package_receipts
    ADD CONSTRAINT document_package_receipts_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.document_reading_events
    ADD CONSTRAINT document_reading_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.document_reading_results
    ADD CONSTRAINT document_reading_results_pkey PRIMARY KEY (document_id);

ALTER TABLE ONLY public.document_readings
    ADD CONSTRAINT document_readings_module_entity_id_sha256_key UNIQUE (module, entity_id, sha256);

ALTER TABLE ONLY public.document_readings
    ADD CONSTRAINT document_readings_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.epcerp_schema_identity
    ADD CONSTRAINT epcerp_schema_identity_pkey PRIMARY KEY (singleton);

ALTER TABLE ONLY public.finance_accounts_acceptance_events
    ADD CONSTRAINT finance_accounts_acceptance_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.finance_accounts_acceptance
    ADD CONSTRAINT finance_accounts_acceptance_package_id_key UNIQUE (package_id);

ALTER TABLE ONLY public.finance_accounts_acceptance
    ADD CONSTRAINT finance_accounts_acceptance_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.finance_advance_decisions
    ADD CONSTRAINT finance_advance_decisions_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.finance_advance_decisions
    ADD CONSTRAINT finance_advance_decisions_request_id_source_version_key UNIQUE (request_id, source_version);

ALTER TABLE ONLY public.finance_budget_decisions
    ADD CONSTRAINT finance_budget_decisions_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.finance_budget_decisions
    ADD CONSTRAINT finance_budget_decisions_project_id_source_revision_key UNIQUE (project_id, source_revision);

ALTER TABLE ONLY public.finance_quotation_decisions
    ADD CONSTRAINT finance_quotation_decisions_attempt_id_key UNIQUE (attempt_id);

ALTER TABLE ONLY public.finance_quotation_decisions
    ADD CONSTRAINT finance_quotation_decisions_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.indian_cities
    ADD CONSTRAINT indian_cities_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.indian_states
    ADD CONSTRAINT indian_states_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.industries
    ADD CONSTRAINT industries_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.manufacturer_imports
    ADD CONSTRAINT manufacturer_imports_go_id_key UNIQUE (go_id);

ALTER TABLE ONLY public.manufacturer_imports
    ADD CONSTRAINT manufacturer_imports_pkey PRIMARY KEY (source_id);

ALTER TABLE ONLY public.manufacturers
    ADD CONSTRAINT manufacturers_code_key UNIQUE (code);

ALTER TABLE ONLY public.manufacturers_events
    ADD CONSTRAINT manufacturers_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.manufacturers
    ADD CONSTRAINT manufacturers_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.material_catalogue
    ADD CONSTRAINT material_catalogue_code_key UNIQUE (code);

ALTER TABLE ONLY public.material_catalogue
    ADD CONSTRAINT material_catalogue_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.organisation_director_designation
    ADD CONSTRAINT organisation_director_designation_id_key UNIQUE (id);

ALTER TABLE ONLY public.organisation_director_designation
    ADD CONSTRAINT organisation_director_designation_pkey PRIMARY KEY (slot);

ALTER TABLE ONLY public.organisation_director_events
    ADD CONSTRAINT organisation_director_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.project_budget_allocations
    ADD CONSTRAINT project_budget_allocations_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.project_budget_events
    ADD CONSTRAINT project_budget_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.project_budget_heads
    ADD CONSTRAINT project_budget_heads_id_project_id_key UNIQUE (id, project_id);

ALTER TABLE ONLY public.project_budget_heads
    ADD CONSTRAINT project_budget_heads_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.project_budgets
    ADD CONSTRAINT project_budgets_pkey PRIMARY KEY (project_id);

ALTER TABLE ONLY public.project_imports
    ADD CONSTRAINT project_imports_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.project_imports
    ADD CONSTRAINT project_imports_project_id_key UNIQUE (project_id);

ALTER TABLE ONLY public.project_imports
    ADD CONSTRAINT project_imports_source_id_key UNIQUE (source_id);

ALTER TABLE ONLY public.project_manager_history
    ADD CONSTRAINT project_manager_history_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.project_manager_history
    ADD CONSTRAINT project_manager_history_project_id_project_version_key UNIQUE (project_id, project_version);

ALTER TABLE ONLY public.project_material_events
    ADD CONSTRAINT project_material_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.project_material_requirements
    ADD CONSTRAINT project_material_requirements_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.project_material_requirements
    ADD CONSTRAINT project_material_requirements_project_id_engineering_key_key UNIQUE (project_id, engineering_key);

ALTER TABLE ONLY public.project_material_requirements
    ADD CONSTRAINT project_material_requirements_project_id_item_id_key UNIQUE (project_id, item_id);

ALTER TABLE ONLY public.project_metadata_changes
    ADD CONSTRAINT project_metadata_changes_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.project_metadata_changes
    ADD CONSTRAINT project_metadata_changes_project_id_project_version_key UNIQUE (project_id, project_version);

ALTER TABLE ONLY public.project_responsibility_events
    ADD CONSTRAINT project_responsibility_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.project_responsibility_profiles
    ADD CONSTRAINT project_responsibility_profiles_id_project_id_key UNIQUE (id, project_id);

ALTER TABLE ONLY public.project_responsibility_profiles
    ADD CONSTRAINT project_responsibility_profiles_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.project_schedule_changes
    ADD CONSTRAINT project_schedule_changes_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.project_schedule_changes
    ADD CONSTRAINT project_schedule_changes_project_id_project_version_key UNIQUE (project_id, project_version);

ALTER TABLE ONLY public.projects
    ADD CONSTRAINT projects_normalized_name_key UNIQUE (normalized_name);

ALTER TABLE ONLY public.projects
    ADD CONSTRAINT projects_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.projects
    ADD CONSTRAINT projects_project_number_key UNIQUE (project_number);

ALTER TABLE ONLY public.purchase_acceptance_events
    ADD CONSTRAINT purchase_acceptance_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_acceptances
    ADD CONSTRAINT purchase_acceptances_order_id_key UNIQUE (order_id);

ALTER TABLE ONLY public.purchase_acceptances
    ADD CONSTRAINT purchase_acceptances_package_id_key UNIQUE (package_id);

ALTER TABLE ONLY public.purchase_acceptances
    ADD CONSTRAINT purchase_acceptances_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_advance_events
    ADD CONSTRAINT purchase_advance_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_advances
    ADD CONSTRAINT purchase_advances_id_project_id_package_id_key UNIQUE (id, project_id, package_id);

ALTER TABLE ONLY public.purchase_advances
    ADD CONSTRAINT purchase_advances_package_id_key UNIQUE (package_id);

ALTER TABLE ONLY public.purchase_advances
    ADD CONSTRAINT purchase_advances_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_award_handoffs
    ADD CONSTRAINT purchase_award_handoffs_attempt_id_key UNIQUE (attempt_id);

ALTER TABLE ONLY public.purchase_award_handoffs
    ADD CONSTRAINT purchase_award_handoffs_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_bo_events
    ADD CONSTRAINT purchase_bo_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_bo_lines
    ADD CONSTRAINT purchase_bo_lines_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_bo_lists
    ADD CONSTRAINT purchase_bo_lists_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_client_clearance_events
    ADD CONSTRAINT purchase_client_clearance_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_client_clearances
    ADD CONSTRAINT purchase_client_clearances_package_id_key UNIQUE (package_id);

ALTER TABLE ONLY public.purchase_client_clearances
    ADD CONSTRAINT purchase_client_clearances_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_dispatch_events
    ADD CONSTRAINT purchase_dispatch_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_dispatches
    ADD CONSTRAINT purchase_dispatches_package_id_key UNIQUE (package_id);

ALTER TABLE ONLY public.purchase_dispatches
    ADD CONSTRAINT purchase_dispatches_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_engineer_confirmations_events
    ADD CONSTRAINT purchase_engineer_confirmations_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_engineer_confirmations
    ADD CONSTRAINT purchase_engineer_confirmations_package_id_key UNIQUE (package_id);

ALTER TABLE ONLY public.purchase_engineer_confirmations
    ADD CONSTRAINT purchase_engineer_confirmations_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_fms_owner_history
    ADD CONSTRAINT purchase_fms_owner_history_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_fms_owner_history
    ADD CONSTRAINT purchase_fms_owner_history_project_id_workspace_version_key UNIQUE (project_id, workspace_version);

ALTER TABLE ONLY public.purchase_fms_workspaces
    ADD CONSTRAINT purchase_fms_workspaces_pkey PRIMARY KEY (project_id);

ALTER TABLE ONLY public.purchase_grns_events
    ADD CONSTRAINT purchase_grns_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_grns
    ADD CONSTRAINT purchase_grns_package_id_key UNIQUE (package_id);

ALTER TABLE ONLY public.purchase_grns
    ADD CONSTRAINT purchase_grns_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_group_quotations
    ADD CONSTRAINT purchase_group_quotations_pkey PRIMARY KEY (bo_id);

ALTER TABLE ONLY public.purchase_hold_events
    ADD CONSTRAINT purchase_hold_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_holds
    ADD CONSTRAINT purchase_holds_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_indent_dispatch_notices
    ADD CONSTRAINT purchase_indent_dispatch_notices_document_id_recipient_id_key UNIQUE (document_id, recipient_id);

ALTER TABLE ONLY public.purchase_indent_dispatch_notices
    ADD CONSTRAINT purchase_indent_dispatch_notices_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_indent_document_events
    ADD CONSTRAINT purchase_indent_document_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_indent_documents
    ADD CONSTRAINT purchase_indent_documents_group_id_vendor_id_scope_revision_key UNIQUE (group_id, vendor_id, scope_revision);

ALTER TABLE ONLY public.purchase_indent_documents
    ADD CONSTRAINT purchase_indent_documents_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_indent_events
    ADD CONSTRAINT purchase_indent_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_indent_group_entries
    ADD CONSTRAINT purchase_indent_group_entries_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_indent_group_events
    ADD CONSTRAINT purchase_indent_group_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_indent_group_sets
    ADD CONSTRAINT purchase_indent_group_sets_pkey PRIMARY KEY (bo_id);

ALTER TABLE ONLY public.purchase_indent_legacy_terms
    ADD CONSTRAINT purchase_indent_legacy_terms_pkey PRIMARY KEY (document_id);

ALTER TABLE ONLY public.purchase_indent_releases
    ADD CONSTRAINT purchase_indent_releases_pkey PRIMARY KEY (indent_id, demand_id);

ALTER TABLE ONLY public.purchase_indent_templates
    ADD CONSTRAINT purchase_indent_templates_pkey PRIMARY KEY (category_code, template_id, version);

ALTER TABLE ONLY public.purchase_indents
    ADD CONSTRAINT purchase_indents_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_inspection_events
    ADD CONSTRAINT purchase_inspection_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_inspection_report_events
    ADD CONSTRAINT purchase_inspection_report_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_inspection_reports
    ADD CONSTRAINT purchase_inspection_reports_package_id_key UNIQUE (package_id);

ALTER TABLE ONLY public.purchase_inspection_reports
    ADD CONSTRAINT purchase_inspection_reports_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_inspections
    ADD CONSTRAINT purchase_inspections_package_id_key UNIQUE (package_id);

ALTER TABLE ONLY public.purchase_inspections
    ADD CONSTRAINT purchase_inspections_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_loi_document_events
    ADD CONSTRAINT purchase_loi_document_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_loi_documents
    ADD CONSTRAINT purchase_loi_documents_handoff_id_request_hash_key UNIQUE (handoff_id, request_hash);

ALTER TABLE ONLY public.purchase_loi_documents
    ADD CONSTRAINT purchase_loi_documents_handoff_id_version_key UNIQUE (handoff_id, version);

ALTER TABLE ONLY public.purchase_loi_documents
    ADD CONSTRAINT purchase_loi_documents_object_key_key UNIQUE (object_key);

ALTER TABLE ONLY public.purchase_loi_documents
    ADD CONSTRAINT purchase_loi_documents_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_loi_reviews
    ADD CONSTRAINT purchase_loi_reviews_document_id_key UNIQUE (document_id);

ALTER TABLE ONLY public.purchase_loi_reviews
    ADD CONSTRAINT purchase_loi_reviews_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_order_client_file_events
    ADD CONSTRAINT purchase_order_client_file_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_order_client_files
    ADD CONSTRAINT purchase_order_client_files_object_key_key UNIQUE (object_key);

ALTER TABLE ONLY public.purchase_order_client_files
    ADD CONSTRAINT purchase_order_client_files_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_order_document_events
    ADD CONSTRAINT purchase_order_document_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_order_events
    ADD CONSTRAINT purchase_order_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_order_sequences
    ADD CONSTRAINT purchase_order_sequences_pkey PRIMARY KEY (fiscal_year);

ALTER TABLE ONLY public.purchase_orders
    ADD CONSTRAINT purchase_orders_id_project_id_key UNIQUE (id, project_id);

ALTER TABLE ONLY public.purchase_orders
    ADD CONSTRAINT purchase_orders_number_key UNIQUE (number);

ALTER TABLE ONLY public.purchase_orders
    ADD CONSTRAINT purchase_orders_package_id_key UNIQUE (package_id);

ALTER TABLE ONLY public.purchase_orders
    ADD CONSTRAINT purchase_orders_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_project_vendors
    ADD CONSTRAINT purchase_project_vendors_pkey PRIMARY KEY (project_id);

ALTER TABLE ONLY public.purchase_quality_cases
    ADD CONSTRAINT purchase_quality_cases_package_id_key UNIQUE (package_id);

ALTER TABLE ONLY public.purchase_quality_cases
    ADD CONSTRAINT purchase_quality_cases_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_quality_certificates
    ADD CONSTRAINT purchase_quality_certificates_pkey PRIMARY KEY (case_id, code);

ALTER TABLE ONLY public.purchase_quality_events
    ADD CONSTRAINT purchase_quality_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_quotation_attachments
    ADD CONSTRAINT purchase_quotation_attachments_document_id_id_key UNIQUE (document_id, id);

ALTER TABLE ONLY public.purchase_quotation_attachments
    ADD CONSTRAINT purchase_quotation_attachments_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_quotation_events
    ADD CONSTRAINT purchase_quotation_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_quotation_evidence_events
    ADD CONSTRAINT purchase_quotation_evidence_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_quotation_revisions
    ADD CONSTRAINT purchase_quotation_revisions_attachment_id_key UNIQUE (attachment_id);

ALTER TABLE ONLY public.purchase_quotation_revisions
    ADD CONSTRAINT purchase_quotation_revisions_pkey PRIMARY KEY (document_id, revision);

ALTER TABLE ONLY public.purchase_quotation_shortlist_drafts
    ADD CONSTRAINT purchase_quotation_shortlist_drafts_pkey PRIMARY KEY (project_id, bo_id, group_id);

ALTER TABLE ONLY public.purchase_quotation_shortlist_history
    ADD CONSTRAINT purchase_quotation_shortlist_history_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_quotation_work_tasks
    ADD CONSTRAINT purchase_quotation_work_tasks_attempt_id_recipient_id_kind_key UNIQUE (attempt_id, recipient_id, kind);

ALTER TABLE ONLY public.purchase_quotation_work_tasks
    ADD CONSTRAINT purchase_quotation_work_tasks_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_site_receipts_events
    ADD CONSTRAINT purchase_site_receipts_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_site_receipts
    ADD CONSTRAINT purchase_site_receipts_package_id_key UNIQUE (package_id);

ALTER TABLE ONLY public.purchase_site_receipts
    ADD CONSTRAINT purchase_site_receipts_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_sourcing_allocations
    ADD CONSTRAINT purchase_sourcing_allocations_pkey PRIMARY KEY (case_id);

ALTER TABLE ONLY public.purchase_sourcing_cases
    ADD CONSTRAINT purchase_sourcing_cases_id_project_id_key UNIQUE (id, project_id);

ALTER TABLE ONLY public.purchase_sourcing_cases
    ADD CONSTRAINT purchase_sourcing_cases_package_id_key UNIQUE (package_id);

ALTER TABLE ONLY public.purchase_sourcing_cases
    ADD CONSTRAINT purchase_sourcing_cases_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_sourcing_events
    ADD CONSTRAINT purchase_sourcing_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_technical_cases
    ADD CONSTRAINT purchase_technical_cases_id_project_id_key UNIQUE (id, project_id);

ALTER TABLE ONLY public.purchase_technical_cases
    ADD CONSTRAINT purchase_technical_cases_package_id_key UNIQUE (package_id);

ALTER TABLE ONLY public.purchase_technical_cases
    ADD CONSTRAINT purchase_technical_cases_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_technical_events
    ADD CONSTRAINT purchase_technical_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_technical_file_events
    ADD CONSTRAINT purchase_technical_file_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_technical_files
    ADD CONSTRAINT purchase_technical_files_id_case_id_key UNIQUE (id, case_id);

ALTER TABLE ONLY public.purchase_technical_files
    ADD CONSTRAINT purchase_technical_files_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_type_indent_drafts
    ADD CONSTRAINT purchase_type_indent_drafts_pkey PRIMARY KEY (project_id, category_code);

ALTER TABLE ONLY public.purchase_work_package_events
    ADD CONSTRAINT purchase_work_package_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_work_packages
    ADD CONSTRAINT purchase_work_packages_code_key UNIQUE (code);

ALTER TABLE ONLY public.purchase_work_packages
    ADD CONSTRAINT purchase_work_packages_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.purchase_work_packages
    ADD CONSTRAINT purchase_work_packages_project_id_indent_id_demand_id_key UNIQUE (project_id, indent_id, demand_id);

ALTER TABLE ONLY public.quotation_approval_events
    ADD CONSTRAINT quotation_approval_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.quotation_approval_sets
    ADD CONSTRAINT quotation_approval_sets_bo_id_project_id_key UNIQUE (bo_id, project_id);

ALTER TABLE ONLY public.quotation_approval_sets
    ADD CONSTRAINT quotation_approval_sets_pkey PRIMARY KEY (bo_id);

ALTER TABLE ONLY public.stored_document_events
    ADD CONSTRAINT stored_document_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.stored_documents
    ADD CONSTRAINT stored_documents_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.supplier_changes
    ADD CONSTRAINT supplier_changes_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.supplier_changes
    ADD CONSTRAINT supplier_changes_supplier_id_version_key UNIQUE (supplier_id, version);

ALTER TABLE ONLY public.supplier_document_approval_changes
    ADD CONSTRAINT supplier_document_approval_changes_document_id_version_key UNIQUE (document_id, version);

ALTER TABLE ONLY public.supplier_document_approval_changes
    ADD CONSTRAINT supplier_document_approval_changes_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.supplier_document_approvals
    ADD CONSTRAINT supplier_document_approvals_pkey PRIMARY KEY (document_id);

ALTER TABLE ONLY public.supplier_document_changes
    ADD CONSTRAINT supplier_document_changes_document_id_version_key UNIQUE (document_id, version);

ALTER TABLE ONLY public.supplier_document_changes
    ADD CONSTRAINT supplier_document_changes_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.supplier_document_tasks
    ADD CONSTRAINT supplier_document_tasks_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.supplier_documents
    ADD CONSTRAINT supplier_documents_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.supplier_documents
    ADD CONSTRAINT supplier_documents_source_id_key UNIQUE (source_id);

ALTER TABLE ONLY public.supplier_review_assignment_changes
    ADD CONSTRAINT supplier_review_assignment_changes_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.supplier_review_assignment_changes
    ADD CONSTRAINT supplier_review_assignment_changes_supplier_id_version_key UNIQUE (supplier_id, version);

ALTER TABLE ONLY public.supplier_review_assignments
    ADD CONSTRAINT supplier_review_assignments_id_key UNIQUE (id);

ALTER TABLE ONLY public.supplier_review_assignments
    ADD CONSTRAINT supplier_review_assignments_pkey PRIMARY KEY (supplier_id);

ALTER TABLE ONLY public.supplier_verification_history
    ADD CONSTRAINT supplier_verification_history_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.supplier_verification_history
    ADD CONSTRAINT supplier_verification_history_supplier_id_version_key UNIQUE (supplier_id, version);

ALTER TABLE ONLY public.supplier_verifications
    ADD CONSTRAINT supplier_verifications_pkey PRIMARY KEY (supplier_id);

ALTER TABLE ONLY public.suppliers
    ADD CONSTRAINT suppliers_code_key UNIQUE (code);

ALTER TABLE ONLY public.suppliers
    ADD CONSTRAINT suppliers_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.suppliers
    ADD CONSTRAINT suppliers_source_id_key UNIQUE (source_id);

ALTER TABLE ONLY public.template_fields
    ADD CONSTRAINT template_fields_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.template_fields
    ADD CONSTRAINT template_fields_template_id_field_code_key UNIQUE (template_id, field_code);

ALTER TABLE ONLY public.template_fields
    ADD CONSTRAINT template_fields_template_id_sort_order_key UNIQUE (template_id, sort_order);

ALTER TABLE ONLY public.template_options
    ADD CONSTRAINT template_options_field_id_option_code_key UNIQUE (field_id, option_code);

ALTER TABLE ONLY public.template_options
    ADD CONSTRAINT template_options_field_id_sort_order_key UNIQUE (field_id, sort_order);

ALTER TABLE ONLY public.template_options
    ADD CONSTRAINT template_options_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.template_publications
    ADD CONSTRAINT template_publications_pkey PRIMARY KEY (template_id);

ALTER TABLE ONLY public.client_types
    ADD CONSTRAINT uq_client_types_code UNIQUE (code);

ALTER TABLE ONLY public.client_types
    ADD CONSTRAINT uq_client_types_normalized_name UNIQUE (normalized_name);

ALTER TABLE ONLY public.indian_cities
    ADD CONSTRAINT uq_indian_cities_state_normalized_name UNIQUE (state_id, normalized_name);

ALTER TABLE ONLY public.indian_states
    ADD CONSTRAINT uq_indian_states_code UNIQUE (code);

ALTER TABLE ONLY public.indian_states
    ADD CONSTRAINT uq_indian_states_normalized_name UNIQUE (normalized_name);

ALTER TABLE ONLY public.industries
    ADD CONSTRAINT uq_industries_code UNIQUE (code);

ALTER TABLE ONLY public.industries
    ADD CONSTRAINT uq_industries_normalized_name UNIQUE (normalized_name);

ALTER TABLE ONLY public.work_calendar_changes
    ADD CONSTRAINT work_calendar_changes_holiday_id_version_key UNIQUE (holiday_id, version);

ALTER TABLE ONLY public.work_calendar_changes
    ADD CONSTRAINT work_calendar_changes_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.work_calendar_holidays
    ADD CONSTRAINT work_calendar_holidays_holiday_date_key UNIQUE (holiday_date);

ALTER TABLE ONLY public.work_calendar_holidays
    ADD CONSTRAINT work_calendar_holidays_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.bid_costings
    ADD CONSTRAINT bid_costings_draft_id_key UNIQUE (draft_id);

ALTER TABLE ONLY tender.bid_costings
    ADD CONSTRAINT bid_costings_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.bid_items
    ADD CONSTRAINT bid_items_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.bidding_authorizations
    ADD CONSTRAINT bidding_authorizations_draft_id_key UNIQUE (draft_id);

ALTER TABLE ONLY tender.bidding_authorizations
    ADD CONSTRAINT bidding_authorizations_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.bidding_authorizations
    ADD CONSTRAINT bidding_authorizations_project_id_key UNIQUE (project_id);

ALTER TABLE ONLY tender.document_drafts
    ADD CONSTRAINT document_drafts_owner_id_operation_key_key UNIQUE (owner_id, operation_key);

ALTER TABLE ONLY tender.document_drafts
    ADD CONSTRAINT document_drafts_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.draft_document_removals
    ADD CONSTRAINT draft_document_removals_pkey PRIMARY KEY (draft_id, member_id);

ALTER TABLE ONLY tender.draft_events
    ADD CONSTRAINT draft_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.draft_sources
    ADD CONSTRAINT draft_sources_pkey PRIMARY KEY (member_id);

ALTER TABLE ONLY tender.prebid_queries
    ADD CONSTRAINT prebid_queries_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.tender_capture_operations
    ADD CONSTRAINT tender_capture_operations_pkey PRIMARY KEY (actor_id, command, context_key, idempotency_key);

ALTER TABLE ONLY tender.tender_email_attempts
    ADD CONSTRAINT tender_email_attempts_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.tender_email_jobs
    ADD CONSTRAINT tender_email_jobs_action_id_key UNIQUE (action_id);

ALTER TABLE ONLY tender.tender_email_jobs
    ADD CONSTRAINT tender_email_jobs_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.tender_fms_events
    ADD CONSTRAINT tender_fms_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.tender_fms_intakes
    ADD CONSTRAINT tender_fms_intakes_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.tender_fms_intakes
    ADD CONSTRAINT tender_fms_intakes_receipt_id_key UNIQUE (receipt_id);

ALTER TABLE ONLY tender.tender_fms_manager_assignments
    ADD CONSTRAINT tender_fms_manager_assignment_id_intake_id_manager_id_versi_key UNIQUE (id, intake_id, manager_id, version);

ALTER TABLE ONLY tender.tender_fms_manager_assignments
    ADD CONSTRAINT tender_fms_manager_assignments_intake_id_version_key UNIQUE (intake_id, version);

ALTER TABLE ONLY tender.tender_fms_manager_assignments
    ADD CONSTRAINT tender_fms_manager_assignments_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.tender_fms_owner_changes
    ADD CONSTRAINT tender_fms_owner_changes_intake_id_version_key UNIQUE (intake_id, version);

ALTER TABLE ONLY tender.tender_fms_owner_changes
    ADD CONSTRAINT tender_fms_owner_changes_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.tender_fms_participant_grants
    ADD CONSTRAINT tender_fms_participant_grants_intake_id_user_id_version_key UNIQUE (intake_id, user_id, version);

ALTER TABLE ONLY tender.tender_fms_participant_grants
    ADD CONSTRAINT tender_fms_participant_grants_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.tender_fms_record_events
    ADD CONSTRAINT tender_fms_record_events_intake_id_business_version_key UNIQUE (intake_id, business_version);

ALTER TABLE ONLY tender.tender_fms_record_events
    ADD CONSTRAINT tender_fms_record_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.tender_provision_operations
    ADD CONSTRAINT tender_provision_operations_estimate_id_key UNIQUE (estimate_id);

ALTER TABLE ONLY tender.tender_provision_operations
    ADD CONSTRAINT tender_provision_operations_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.tender_provision_operations
    ADD CONSTRAINT tender_provision_operations_project_id_key UNIQUE (project_id);

ALTER TABLE ONLY tender.tender_provision_operations
    ADD CONSTRAINT tender_provision_operations_tender_id_key UNIQUE (tender_id);

ALTER TABLE ONLY tender.tender_source_actions
    ADD CONSTRAINT tender_source_actions_id_source_event_id_recipient_id_assig_key UNIQUE (id, source_event_id, recipient_id, assignment_version);

ALTER TABLE ONLY tender.tender_source_actions
    ADD CONSTRAINT tender_source_actions_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.tender_source_actions
    ADD CONSTRAINT tender_source_actions_source_event_id_key UNIQUE (source_event_id);

ALTER TABLE ONLY tender.tender_source_actions
    ADD CONSTRAINT tender_source_actions_tender_id_kind_assignment_version_key UNIQUE (tender_id, kind, assignment_version);

ALTER TABLE ONLY tender.tender_source_links
    ADD CONSTRAINT tender_source_links_pkey PRIMARY KEY (member_id);

ALTER TABLE ONLY tender.tsc_cloud_calls
    ADD CONSTRAINT tsc_cloud_calls_draft_id_request_key_key UNIQUE (draft_id, request_key);

ALTER TABLE ONLY tender.tsc_cloud_calls
    ADD CONSTRAINT tsc_cloud_calls_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.tsc_document_text
    ADD CONSTRAINT tsc_document_text_pkey PRIMARY KEY (member_id, reader_version);

ALTER TABLE ONLY tender.tsc_eligibility_verdicts
    ADD CONSTRAINT tsc_eligibility_verdicts_draft_id_job_id_key UNIQUE (draft_id, job_id);

ALTER TABLE ONLY tender.tsc_eligibility_verdicts
    ADD CONSTRAINT tsc_eligibility_verdicts_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.tsc_events
    ADD CONSTRAINT tsc_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.tsc_format_events
    ADD CONSTRAINT tsc_format_events_format_id_version_key UNIQUE (format_id, version);

ALTER TABLE ONLY tender.tsc_format_events
    ADD CONSTRAINT tsc_format_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.tsc_format_questions
    ADD CONSTRAINT tsc_format_questions_format_id_position_key UNIQUE (format_id, "position");

ALTER TABLE ONLY tender.tsc_format_questions
    ADD CONSTRAINT tsc_format_questions_format_id_question_key_key UNIQUE (format_id, question_key);

ALTER TABLE ONLY tender.tsc_format_questions
    ADD CONSTRAINT tsc_format_questions_pkey PRIMARY KEY (format_id, id);

ALTER TABLE ONLY tender.tsc_formats
    ADD CONSTRAINT tsc_formats_name_key_key UNIQUE (name_key);

ALTER TABLE ONLY tender.tsc_formats
    ADD CONSTRAINT tsc_formats_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.tsc_jobs
    ADD CONSTRAINT tsc_jobs_draft_id_actor_id_operation_key_key UNIQUE (draft_id, actor_id, operation_key);

ALTER TABLE ONLY tender.tsc_jobs
    ADD CONSTRAINT tsc_jobs_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.tsc_model_calls
    ADD CONSTRAINT tsc_model_calls_id_job_id_draft_id_key UNIQUE (id, job_id, draft_id);

ALTER TABLE ONLY tender.tsc_model_calls
    ADD CONSTRAINT tsc_model_calls_job_id_request_key_key UNIQUE (job_id, request_key);

ALTER TABLE ONLY tender.tsc_model_calls
    ADD CONSTRAINT tsc_model_calls_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.tsc_model_responses
    ADD CONSTRAINT tsc_model_responses_call_id_key UNIQUE (call_id);

ALTER TABLE ONLY tender.tsc_model_responses
    ADD CONSTRAINT tsc_model_responses_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.tsc_workspace_events
    ADD CONSTRAINT tsc_workspace_events_job_id_version_key UNIQUE (job_id, version);

ALTER TABLE ONLY tender.tsc_workspace_events
    ADD CONSTRAINT tsc_workspace_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.tsc_workspace_tasks
    ADD CONSTRAINT tsc_workspace_tasks_job_id_operation_key_key UNIQUE (job_id, operation_key);

ALTER TABLE ONLY tender.tsc_workspace_tasks
    ADD CONSTRAINT tsc_workspace_tasks_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.tsc_workspaces
    ADD CONSTRAINT tsc_workspaces_draft_id_actor_id_operation_key_key UNIQUE (draft_id, actor_id, operation_key);

ALTER TABLE ONLY tender.tsc_workspaces
    ADD CONSTRAINT tsc_workspaces_id_draft_id_key UNIQUE (id, draft_id);

ALTER TABLE ONLY tender.tsc_workspaces
    ADD CONSTRAINT tsc_workspaces_pkey PRIMARY KEY (id);

ALTER TABLE ONLY tender.vendor_quotations
    ADD CONSTRAINT vendor_quotations_pkey PRIMARY KEY (id);

CREATE INDEX access_sessions_family ON auth.access_sessions USING btree (family_id);

CREATE INDEX auth_credential_emails_pending ON auth.auth_credential_emails USING btree (next_attempt_at) WHERE (state = ANY (ARRAY['pending'::text, 'sending'::text]));

CREATE INDEX auth_credential_emails_user ON auth.auth_credential_emails USING btree (user_id, created_at DESC);

CREATE INDEX auth_manager_offboarding ON auth.auth_offboarding_cases USING btree (manager_id, state);

CREATE UNIQUE INDEX auth_one_open_offboarding ON auth.auth_offboarding_cases USING btree (user_id) WHERE (state = ANY (ARRAY['pending'::text, 'planning'::text, 'approved'::text]));

CREATE INDEX auth_password_recovery_user_created ON auth.auth_password_recovery USING btree (user_id, created_at DESC);

CREATE INDEX auth_reporting_department ON auth.auth_reporting_lines USING btree (department_id) WHERE is_active;

CREATE INDEX auth_reporting_manager ON auth.auth_reporting_lines USING btree (manager_id) WHERE is_active;

CREATE UNIQUE INDEX email_changes_one_pending ON auth.email_changes USING btree (user_id) WHERE (consumed_at IS NULL);

CREATE INDEX refresh_tokens_family_id_idx ON auth.refresh_tokens USING btree (family_id);

CREATE INDEX refresh_tokens_user_id_idx ON auth.refresh_tokens USING btree (user_id);

CREATE INDEX role_permissions_permission_id_idx ON auth.role_permissions USING btree (permission_id);

CREATE INDEX user_roles_role_id_idx ON auth.user_roles USING btree (role_id);

CREATE UNIQUE INDEX users_email_key ON auth.users USING btree (email);

CREATE UNIQUE INDEX users_username_key ON auth.users USING btree (username);

CREATE INDEX activity_events_timeline_idx ON ONLY public.activity_events USING btree (scope_id, occurred_at DESC, id DESC, action);

CREATE INDEX activity_events_project_scope_id_occurred_at_id_action_idx ON public.activity_events_project USING btree (scope_id, occurred_at DESC, id DESC, action);

CREATE UNIQUE INDEX activity_events_purchase_start_idx ON ONLY public.activity_events USING btree (scope_kind, scope_id) WHERE ((module = 'purchase'::text) AND (action = 'purchase_started'::text));

CREATE UNIQUE INDEX activity_events_project_scope_kind_scope_id_idx ON public.activity_events_project USING btree (scope_kind, scope_id) WHERE ((module = 'purchase'::text) AND (action = 'purchase_started'::text));

CREATE INDEX activity_events_tender_scope_id_occurred_at_id_action_idx ON public.activity_events_tender USING btree (scope_id, occurred_at DESC, id DESC, action);

CREATE UNIQUE INDEX activity_events_tender_scope_kind_scope_id_idx ON public.activity_events_tender USING btree (scope_kind, scope_id) WHERE ((module = 'purchase'::text) AND (action = 'purchase_started'::text));

CREATE INDEX business_events_actor_idx ON public.business_events USING btree (actor_id) WHERE (actor_id IS NOT NULL);

CREATE INDEX business_events_scope_idx ON public.business_events USING btree (scope_kind, scope_id, recorded_at, id);

CREATE UNIQUE INDEX catalogue_items_category_code_serial ON public.catalogue_items USING btree ((((snapshot -> 'configuration'::text) ->> 'category_code'::text)), serial);

CREATE INDEX catalogue_items_engineering ON public.catalogue_items USING btree (engineering_key);

CREATE INDEX category_subcategories_parent ON public.category_subcategories USING btree (category_id);

CREATE INDEX client_contacts_client_id_idx ON public.client_contacts USING btree (client_id);

CREATE INDEX client_purchase_rules_client ON public.client_purchase_rules USING btree (client_id);

CREATE UNIQUE INDEX clients_client_code_key ON public.clients USING btree (client_code);

CREATE UNIQUE INDEX clients_normalized_legal_name_key ON public.clients USING btree (normalized_legal_name);

CREATE INDEX communication_recipient_list ON public.communication_notices USING btree (recipient_id, created_at DESC, id DESC);

CREATE INDEX document_package_claims ON public.document_package_receipts USING btree (state, lease_until, created_at) WHERE (state = 'verifying'::text);

CREATE INDEX document_readings_queue ON public.document_readings USING btree (status, created_at) WHERE ((status)::text = ANY ((ARRAY['queued'::character varying, 'processing'::character varying])::text[]));

CREATE INDEX document_readings_scope ON public.document_readings USING btree (module, entity_id);

CREATE INDEX finance_accounts_acceptance_history ON public.finance_accounts_acceptance_events USING btree (subject_id, created_at DESC);

CREATE INDEX manufacturers_events_subject_id_idx ON public.manufacturers_events USING btree (subject_id, created_at);

CREATE INDEX organisation_director_history ON public.organisation_director_events USING btree (created_at, id);

CREATE INDEX package_draft_ready ON public.document_package_receipts USING btree (draft_id, state) WHERE (draft_id IS NOT NULL);

CREATE INDEX package_manifest_claim ON public.document_package_manifest USING btree (receipt_id, claim_id, created_at);

CREATE UNIQUE INDEX project_budget_active_item ON public.project_budget_allocations USING btree (project_id, bo_id, line_id) WHERE is_active;

CREATE INDEX project_budget_head_project ON public.project_budget_heads USING btree (project_id);

CREATE INDEX project_budget_history ON public.project_budget_events USING btree (project_id, created_at, id);

CREATE INDEX project_manager_history_actor_idx ON public.project_manager_history USING btree (created_by_id) WHERE (created_by_id IS NOT NULL);

CREATE INDEX project_manager_history_manager_idx ON public.project_manager_history USING btree (project_manager_id);

CREATE INDEX project_manager_history_previous_idx ON public.project_manager_history USING btree (previous_manager_id) WHERE (previous_manager_id IS NOT NULL);

CREATE INDEX project_manager_history_project_idx ON public.project_manager_history USING btree (project_id, project_version DESC);

CREATE UNIQUE INDEX project_responsibility_active_identity ON public.project_responsibility_profiles USING btree (project_id, user_id, bundle_id, responsibility_key) WHERE is_active;

CREATE INDEX project_responsibility_history ON public.project_responsibility_events USING btree (profile_id, created_at, id);

CREATE INDEX project_responsibility_scope ON public.project_responsibility_profiles USING btree (project_id, bundle_id, id);

CREATE INDEX projects_client_idx ON public.projects USING btree (client_id);

CREATE INDEX projects_created_actor_idx ON public.projects USING btree (created_by_id) WHERE (created_by_id IS NOT NULL);

CREATE INDEX projects_manager_idx ON public.projects USING btree (project_manager_id, created_at DESC, id);

CREATE INDEX projects_updated_actor_idx ON public.projects USING btree (updated_by_id) WHERE (updated_by_id IS NOT NULL);

CREATE INDEX purchase_acceptance_history ON public.purchase_acceptance_events USING btree (subject_id, created_at DESC);

CREATE UNIQUE INDEX purchase_advance_event_version ON public.purchase_advance_events USING btree (subject_id, ((after_snapshot ->> 'version'::text)));

CREATE INDEX purchase_advance_history ON public.purchase_advance_events USING btree (subject_id, created_at DESC);

CREATE INDEX purchase_bo_project ON public.purchase_bo_lists USING btree (project_id);

CREATE INDEX purchase_client_clearance_history ON public.purchase_client_clearance_events USING btree (subject_id, created_at DESC);

CREATE INDEX purchase_dispatch_history ON public.purchase_dispatch_events USING btree (subject_id, created_at DESC);

CREATE INDEX purchase_engineer_confirmations_history ON public.purchase_engineer_confirmations_events USING btree (subject_id, created_at DESC);

CREATE INDEX purchase_fms_owner_projects ON public.purchase_fms_workspaces USING btree (purchase_manager_id, project_id) WHERE is_active;

CREATE INDEX purchase_grns_history ON public.purchase_grns_events USING btree (subject_id, created_at DESC);

CREATE INDEX purchase_holds_project ON public.purchase_holds USING btree (project_id);

CREATE INDEX purchase_indent_document_scope ON public.purchase_indent_documents USING btree (project_id, bo_id);

CREATE INDEX purchase_indent_group_project ON public.purchase_indent_group_sets USING btree (project_id);

CREATE INDEX purchase_indent_notice_recipient ON public.purchase_indent_dispatch_notices USING btree (recipient_id, created_at DESC) WHERE (resolved_at IS NULL);

CREATE INDEX purchase_indents_project ON public.purchase_indents USING btree (project_id);

CREATE INDEX purchase_inspection_history ON public.purchase_inspection_events USING btree (subject_id, created_at DESC);

CREATE INDEX purchase_inspection_report_history ON public.purchase_inspection_report_events USING btree (subject_id, created_at DESC);

CREATE INDEX purchase_loi_project_attempt ON public.purchase_loi_documents USING btree (project_id, attempt_id, version DESC);

CREATE INDEX purchase_order_history ON public.purchase_order_events USING btree (subject_id, created_at DESC);

CREATE INDEX purchase_quality_history ON public.purchase_quality_events USING btree (subject_id, created_at DESC);

CREATE INDEX purchase_quotation_attachment_document ON public.purchase_quotation_attachments USING btree (document_id, created_at);

CREATE INDEX purchase_quotation_project ON public.purchase_group_quotations USING btree (project_id, bo_id);

CREATE INDEX purchase_quotation_revision_total ON public.purchase_quotation_revisions USING btree (document_id, invoice_total);

CREATE INDEX purchase_quotation_work_recipient ON public.purchase_quotation_work_tasks USING btree (recipient_id, created_at DESC) WHERE (state <> 'completed'::text);

CREATE INDEX purchase_site_receipts_history ON public.purchase_site_receipts_events USING btree (subject_id, created_at DESC);

CREATE INDEX purchase_sourcing_allocation_balance ON public.purchase_sourcing_allocations USING btree (project_id, demand_id) WHERE is_active;

CREATE INDEX purchase_sourcing_project_scope ON public.purchase_sourcing_cases USING btree (project_id, package_id);

CREATE INDEX purchase_technical_history ON public.purchase_technical_events USING btree (subject_id, created_at DESC);

CREATE INDEX purchase_work_packages_assignment_lookup ON public.purchase_work_packages USING gin (steps jsonb_path_ops) WHERE is_active;

CREATE INDEX purchase_work_packages_project ON public.purchase_work_packages USING btree (project_id, created_at, id);

CREATE INDEX stored_document_events_document_id_idx ON public.stored_document_events USING btree (document_id, created_at);

CREATE INDEX stored_documents_entity_idx ON public.stored_documents USING btree (entity_type, entity_id);

CREATE UNIQUE INDEX stored_documents_object_key ON public.stored_documents USING btree (storage_bucket, storage_key);

CREATE UNIQUE INDEX supplier_document_pending_task ON public.supplier_document_tasks USING btree (document_id, kind) WHERE (completed_at IS NULL);

CREATE INDEX supplier_document_user_tasks ON public.supplier_document_tasks USING btree (user_id, completed_at, created_at);

CREATE UNIQUE INDEX supplier_documents_object_identity ON public.supplier_documents USING btree (object_bucket, object_key) WHERE (object_key IS NOT NULL);

CREATE INDEX supplier_documents_supplier_idx ON public.supplier_documents USING btree (supplier_id, created_at, id);

CREATE INDEX supplier_review_assignee ON public.supplier_review_assignments USING btree (user_id, supplier_id) WHERE is_active;

CREATE UNIQUE INDEX technical_object_key ON public.purchase_technical_files USING btree (object_bucket, object_key) WHERE (object_key <> ''::text);

CREATE UNIQUE INDEX template_description_order ON public.template_fields USING btree (template_id, description_order) WHERE use_in_description;

CREATE INDEX document_draft_owner ON tender.document_drafts USING btree (owner_id, created_at DESC);

CREATE UNIQUE INDEX tender_fms_legacy_fingerprint ON tender.tender_fms_intakes USING btree (owner_id, initial_sha256) WHERE (record_kind = 'legacy'::text);

CREATE INDEX tender_fms_sales_lookup ON tender.tender_fms_intakes USING btree (sales_creator_id, id) WHERE (record_kind = 'named'::text);

CREATE INDEX tsc_claim_queue ON tender.tsc_jobs USING btree (state, created_at);

CREATE INDEX tsc_cloud_draft_budget ON tender.tsc_cloud_calls USING btree (draft_id);

CREATE UNIQUE INDEX tsc_one_pending ON tender.tsc_jobs USING btree (draft_id) WHERE (state = ANY (ARRAY['queued'::text, 'processing'::text]));

CREATE UNIQUE INDEX tsc_task_pending ON tender.tsc_workspace_tasks USING btree (job_id) WHERE (state = ANY (ARRAY['queued'::text, 'processing'::text]));

CREATE UNIQUE INDEX tsc_workspace_pending ON tender.tsc_workspaces USING btree (draft_id) WHERE (state = ANY (ARRAY['queued'::text, 'processing'::text]));

ALTER INDEX public.activity_events_pkey ATTACH PARTITION public.activity_events_project_pkey;

ALTER INDEX public.activity_events_timeline_idx ATTACH PARTITION public.activity_events_project_scope_id_occurred_at_id_action_idx;

ALTER INDEX public.activity_events_purchase_start_idx ATTACH PARTITION public.activity_events_project_scope_kind_scope_id_idx;

ALTER INDEX public.activity_events_pkey ATTACH PARTITION public.activity_events_tender_pkey;

ALTER INDEX public.activity_events_timeline_idx ATTACH PARTITION public.activity_events_tender_scope_id_occurred_at_id_action_idx;

ALTER INDEX public.activity_events_purchase_start_idx ATTACH PARTITION public.activity_events_tender_scope_kind_scope_id_idx;

CREATE TRIGGER administration_changes_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON auth.administration_changes FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER auth_bootstrap_identity_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON auth.auth_bootstrap_identity FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER auth_credential_changes_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON auth.auth_credential_changes FOR EACH STATEMENT EXECUTE FUNCTION auth.reject_auth_credential_change_mutation();

CREATE TRIGGER auth_offboarding_changes_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON auth.auth_offboarding_changes FOR EACH STATEMENT EXECUTE FUNCTION auth.reject_auth_organisation_change_mutation();

CREATE TRIGGER auth_organisation_changes_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON auth.auth_organisation_changes FOR EACH STATEMENT EXECUTE FUNCTION auth.reject_auth_organisation_change_mutation();

CREATE TRIGGER auth_personal_contact_changes_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON auth.auth_personal_contact_changes FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER auth_reporting_line_guard BEFORE INSERT OR UPDATE ON auth.auth_reporting_lines FOR EACH ROW EXECUTE FUNCTION auth.guard_auth_reporting_line();

CREATE TRIGGER auth_user_status_changes_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON auth.auth_user_status_changes FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER auth_username_changes_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON auth.auth_username_changes FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER role_permissions_administration_audit AFTER INSERT OR DELETE ON auth.role_permissions FOR EACH ROW EXECUTE FUNCTION auth.audit_administration();

CREATE TRIGGER role_permissions_version AFTER INSERT OR DELETE ON auth.role_permissions FOR EACH ROW EXECUTE FUNCTION auth.advance_assignment_version();

CREATE TRIGGER roles_administration_audit AFTER INSERT OR UPDATE ON auth.roles FOR EACH ROW EXECUTE FUNCTION auth.audit_administration();

CREATE TRIGGER roles_edit_version BEFORE UPDATE ON auth.roles FOR EACH ROW EXECUTE FUNCTION auth.advance_edit_version();

CREATE TRIGGER user_roles_administration_audit AFTER INSERT OR DELETE ON auth.user_roles FOR EACH ROW EXECUTE FUNCTION auth.audit_administration();

CREATE TRIGGER user_roles_version AFTER INSERT OR DELETE ON auth.user_roles FOR EACH ROW EXECUTE FUNCTION auth.advance_assignment_version();

CREATE TRIGGER users_administration_audit AFTER INSERT OR UPDATE ON auth.users FOR EACH ROW EXECUTE FUNCTION auth.audit_administration();

CREATE TRIGGER users_edit_version BEFORE UPDATE ON auth.users FOR EACH ROW EXECUTE FUNCTION auth.advance_edit_version();

CREATE TRIGGER award_handoff_immutable BEFORE DELETE OR UPDATE ON public.purchase_award_handoffs FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_sourcing_history();

CREATE CONSTRAINT TRIGGER award_handoff_integrity AFTER INSERT ON public.purchase_award_handoffs DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_award_handoff();

CREATE TRIGGER award_handoff_no_truncate BEFORE TRUNCATE ON public.purchase_award_handoffs FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_sourcing_history();

CREATE TRIGGER business_events_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON public.business_events FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER calendar_history_immutable BEFORE DELETE OR UPDATE ON public.work_calendar_changes FOR EACH ROW EXECUTE FUNCTION public.reject_calendar_history_change();

CREATE TRIGGER calendar_history_no_truncate BEFORE TRUNCATE ON public.work_calendar_changes FOR EACH STATEMENT EXECUTE FUNCTION public.reject_calendar_history_change();

CREATE TRIGGER catalogue_item_events_immutable BEFORE DELETE OR UPDATE ON public.catalogue_item_events FOR EACH ROW EXECUTE FUNCTION public.protect_catalogue_item_evidence();

CREATE TRIGGER catalogue_item_identity BEFORE UPDATE ON public.catalogue_items FOR EACH ROW EXECUTE FUNCTION public.protect_catalogue_item_identity();

CREATE TRIGGER catalogue_item_no_delete BEFORE DELETE ON public.catalogue_items FOR EACH ROW EXECUTE FUNCTION public.protect_catalogue_item_evidence();

CREATE TRIGGER category_catalogue_changes_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON public.category_catalogue_changes FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER client_purchase_rule_events_immutable BEFORE DELETE OR UPDATE ON public.client_purchase_rule_events FOR EACH ROW EXECUTE FUNCTION public.protect_client_purchase_rule_history();

CREATE TRIGGER client_purchase_rule_events_no_truncate BEFORE TRUNCATE ON public.client_purchase_rule_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_client_purchase_rule_history();

CREATE TRIGGER client_purchase_rules_no_delete BEFORE DELETE ON public.client_purchase_rules FOR EACH ROW EXECUTE FUNCTION public.protect_client_purchase_rule_history();

CREATE TRIGGER client_purchase_rules_no_truncate BEFORE TRUNCATE ON public.client_purchase_rules FOR EACH STATEMENT EXECUTE FUNCTION public.protect_client_purchase_rule_history();

CREATE TRIGGER communication_notice_events_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON public.communication_notice_events FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER communication_notice_identity BEFORE INSERT OR UPDATE ON public.communication_notices FOR EACH ROW EXECUTE FUNCTION public.protect_communication_notice();

CREATE TRIGGER communication_notices_no_delete BEFORE DELETE OR TRUNCATE ON public.communication_notices FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER document_package_events_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON public.document_package_events FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER document_package_file_identity BEFORE DELETE OR UPDATE ON public.document_package_files FOR EACH ROW EXECUTE FUNCTION public.protect_package_file();

CREATE TRIGGER document_package_files_no_truncate BEFORE TRUNCATE ON public.document_package_files FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER document_package_identity BEFORE DELETE OR UPDATE ON public.document_package_receipts FOR EACH ROW EXECUTE FUNCTION public.protect_consumed_package();

CREATE TRIGGER document_package_members_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON public.document_package_members FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER document_package_no_truncate BEFORE TRUNCATE ON public.document_package_receipts FOR EACH STATEMENT EXECUTE FUNCTION public.protect_consumed_package();

CREATE TRIGGER document_package_operations_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON public.document_package_operations FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER finance_accounts_acceptance_event_immutable BEFORE DELETE OR UPDATE ON public.finance_accounts_acceptance_events FOR EACH ROW EXECUTE FUNCTION public.protect_finance_accounts_acceptance_history();

CREATE CONSTRAINT TRIGGER finance_accounts_acceptance_event_integrity AFTER INSERT ON public.finance_accounts_acceptance_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_finance_accounts_acceptance_event();

CREATE TRIGGER finance_accounts_acceptance_event_no_truncate BEFORE TRUNCATE ON public.finance_accounts_acceptance_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_finance_accounts_acceptance_history();

CREATE TRIGGER finance_accounts_acceptance_guard BEFORE INSERT OR UPDATE ON public.finance_accounts_acceptance FOR EACH ROW EXECUTE FUNCTION public.protect_finance_accounts_acceptance();

CREATE TRIGGER finance_accounts_acceptance_no_delete BEFORE DELETE ON public.finance_accounts_acceptance FOR EACH ROW EXECUTE FUNCTION public.protect_finance_accounts_acceptance_history();

CREATE TRIGGER finance_accounts_acceptance_no_truncate BEFORE TRUNCATE ON public.finance_accounts_acceptance FOR EACH STATEMENT EXECUTE FUNCTION public.protect_finance_accounts_acceptance_history();

CREATE CONSTRAINT TRIGGER finance_accounts_acceptance_write_integrity AFTER INSERT OR UPDATE ON public.finance_accounts_acceptance DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_finance_accounts_acceptance_write();

CREATE CONSTRAINT TRIGGER finance_advance_event AFTER INSERT ON public.finance_advance_decisions DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_finance_advance_event();

CREATE TRIGGER finance_advance_immutable BEFORE DELETE OR UPDATE ON public.finance_advance_decisions FOR EACH ROW EXECUTE FUNCTION public.protect_finance_budget_decisions();

CREATE TRIGGER finance_advance_no_truncate BEFORE TRUNCATE ON public.finance_advance_decisions FOR EACH STATEMENT EXECUTE FUNCTION public.protect_finance_budget_decisions();

CREATE TRIGGER finance_advance_source BEFORE INSERT ON public.finance_advance_decisions FOR EACH ROW EXECUTE FUNCTION public.protect_finance_advance_source();

CREATE TRIGGER finance_budget_decision_immutable BEFORE DELETE OR UPDATE ON public.finance_budget_decisions FOR EACH ROW EXECUTE FUNCTION public.protect_finance_budget_decisions();

CREATE TRIGGER finance_budget_decision_no_truncate BEFORE TRUNCATE ON public.finance_budget_decisions FOR EACH STATEMENT EXECUTE FUNCTION public.protect_finance_budget_decisions();

CREATE CONSTRAINT TRIGGER finance_budget_event AFTER INSERT ON public.finance_budget_decisions DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_finance_budget_event();

CREATE CONSTRAINT TRIGGER finance_quotation_event AFTER INSERT ON public.finance_quotation_decisions DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_finance_quotation_event();

CREATE TRIGGER finance_quotation_immutable BEFORE DELETE OR UPDATE ON public.finance_quotation_decisions FOR EACH ROW EXECUTE FUNCTION public.protect_finance_budget_decisions();

CREATE TRIGGER finance_quotation_no_truncate BEFORE TRUNCATE ON public.finance_quotation_decisions FOR EACH STATEMENT EXECUTE FUNCTION public.protect_finance_budget_decisions();

CREATE TRIGGER immutable_manufacturer_event BEFORE DELETE OR UPDATE ON public.manufacturers_events FOR EACH ROW EXECUTE FUNCTION public.protect_manufacturer_evidence();

CREATE TRIGGER immutable_manufacturer_import BEFORE DELETE OR UPDATE ON public.manufacturer_imports FOR EACH ROW EXECUTE FUNCTION public.protect_manufacturer_evidence();

CREATE TRIGGER immutable_publication BEFORE DELETE OR UPDATE ON public.template_publications FOR EACH ROW EXECUTE FUNCTION public.protect_template_revision();

CREATE TRIGGER immutable_reading_event BEFORE DELETE OR UPDATE ON public.document_reading_events FOR EACH ROW EXECUTE FUNCTION public.protect_reading_evidence();

CREATE TRIGGER immutable_reading_result BEFORE DELETE OR UPDATE ON public.document_reading_results FOR EACH ROW EXECUTE FUNCTION public.protect_reading_evidence();

CREATE TRIGGER immutable_reading_source BEFORE DELETE OR UPDATE ON public.document_readings FOR EACH ROW EXECUTE FUNCTION public.protect_reading_source();

CREATE TRIGGER immutable_template BEFORE DELETE OR UPDATE ON public.configuration_templates FOR EACH ROW EXECUTE FUNCTION public.protect_template_revision();

CREATE TRIGGER immutable_template_field BEFORE DELETE OR UPDATE ON public.template_fields FOR EACH ROW EXECUTE FUNCTION public.protect_template_revision();

CREATE TRIGGER immutable_template_option BEFORE DELETE OR UPDATE ON public.template_options FOR EACH ROW EXECUTE FUNCTION public.protect_template_revision();

CREATE TRIGGER organisation_director_events_immutable BEFORE DELETE OR UPDATE ON public.organisation_director_events FOR EACH ROW EXECUTE FUNCTION public.protect_organisation_director_evidence();

CREATE TRIGGER organisation_director_events_no_truncate BEFORE TRUNCATE ON public.organisation_director_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_organisation_director_evidence();

CREATE TRIGGER organisation_director_identity BEFORE UPDATE ON public.organisation_director_designation FOR EACH ROW EXECUTE FUNCTION public.protect_organisation_director_identity();

CREATE TRIGGER organisation_director_no_delete BEFORE DELETE ON public.organisation_director_designation FOR EACH ROW EXECUTE FUNCTION public.protect_organisation_director_evidence();

CREATE TRIGGER organisation_director_no_truncate BEFORE TRUNCATE ON public.organisation_director_designation FOR EACH STATEMENT EXECUTE FUNCTION public.protect_organisation_director_evidence();

CREATE TRIGGER package_manifest_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON public.document_package_manifest FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER po_client_file_event_immutable BEFORE DELETE OR UPDATE ON public.purchase_order_client_file_events FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_order_history();

CREATE CONSTRAINT TRIGGER po_client_file_event_integrity AFTER INSERT ON public.purchase_order_client_file_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_po_client_file_event();

CREATE TRIGGER po_client_file_event_no_truncate BEFORE TRUNCATE ON public.purchase_order_client_file_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_order_history();

CREATE TRIGGER po_client_file_immutable BEFORE DELETE OR UPDATE ON public.purchase_order_client_files FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_order_history();

CREATE CONSTRAINT TRIGGER po_client_file_integrity AFTER INSERT ON public.purchase_order_client_files DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_po_client_file_write();

CREATE TRIGGER po_client_file_no_truncate BEFORE TRUNCATE ON public.purchase_order_client_files FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_order_history();

CREATE TRIGGER po_client_signoff_guard BEFORE UPDATE ON public.purchase_orders FOR EACH ROW EXECUTE FUNCTION public.check_po_client_signoff();

CREATE TRIGGER project_budget_allocation_identity BEFORE UPDATE ON public.project_budget_allocations FOR EACH ROW EXECUTE FUNCTION public.protect_project_budget_child_identity();

CREATE TRIGGER project_budget_allocation_scope BEFORE INSERT OR UPDATE ON public.project_budget_allocations FOR EACH ROW EXECUTE FUNCTION public.protect_project_budget_item_scope();

CREATE TRIGGER project_budget_allocations_no_delete BEFORE DELETE ON public.project_budget_allocations FOR EACH ROW EXECUTE FUNCTION public.protect_project_budget_evidence();

CREATE TRIGGER project_budget_allocations_no_truncate BEFORE TRUNCATE ON public.project_budget_allocations FOR EACH STATEMENT EXECUTE FUNCTION public.protect_project_budget_evidence();

CREATE TRIGGER project_budget_events_immutable BEFORE DELETE OR UPDATE ON public.project_budget_events FOR EACH ROW EXECUTE FUNCTION public.protect_project_budget_evidence();

CREATE TRIGGER project_budget_events_no_truncate BEFORE TRUNCATE ON public.project_budget_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_project_budget_evidence();

CREATE TRIGGER project_budget_finance_baseline BEFORE INSERT OR UPDATE ON public.project_budgets FOR EACH ROW EXECUTE FUNCTION public.check_project_budget_finance_baseline();

CREATE TRIGGER project_budget_head_identity BEFORE UPDATE ON public.project_budget_heads FOR EACH ROW EXECUTE FUNCTION public.protect_project_budget_child_identity();

CREATE TRIGGER project_budget_heads_no_delete BEFORE DELETE ON public.project_budget_heads FOR EACH ROW EXECUTE FUNCTION public.protect_project_budget_evidence();

CREATE TRIGGER project_budget_heads_no_truncate BEFORE TRUNCATE ON public.project_budget_heads FOR EACH STATEMENT EXECUTE FUNCTION public.protect_project_budget_evidence();

CREATE TRIGGER project_budget_identity BEFORE UPDATE ON public.project_budgets FOR EACH ROW EXECUTE FUNCTION public.protect_project_budget_identity();

CREATE TRIGGER project_budgets_no_delete BEFORE DELETE ON public.project_budgets FOR EACH ROW EXECUTE FUNCTION public.protect_project_budget_evidence();

CREATE TRIGGER project_budgets_no_truncate BEFORE TRUNCATE ON public.project_budgets FOR EACH STATEMENT EXECUTE FUNCTION public.protect_project_budget_evidence();

CREATE TRIGGER project_import_immutable BEFORE DELETE OR UPDATE ON public.project_imports FOR EACH ROW EXECUTE FUNCTION public.reject_project_import_change();

CREATE TRIGGER project_import_no_truncate BEFORE TRUNCATE ON public.project_imports FOR EACH STATEMENT EXECUTE FUNCTION public.reject_project_import_change();

CREATE TRIGGER project_manager_history_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON public.project_manager_history FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER project_material_events_immutable BEFORE DELETE OR UPDATE ON public.project_material_events FOR EACH ROW EXECUTE FUNCTION public.protect_project_material_history();

CREATE TRIGGER project_material_events_no_truncate BEFORE TRUNCATE ON public.project_material_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_project_material_history();

CREATE TRIGGER project_material_identity BEFORE UPDATE ON public.project_material_requirements FOR EACH ROW EXECUTE FUNCTION public.protect_project_material_identity();

CREATE TRIGGER project_material_no_delete BEFORE DELETE ON public.project_material_requirements FOR EACH ROW EXECUTE FUNCTION public.protect_project_material_history();

CREATE TRIGGER project_material_no_truncate BEFORE TRUNCATE ON public.project_material_requirements FOR EACH STATEMENT EXECUTE FUNCTION public.protect_project_material_history();

CREATE TRIGGER project_metadata_immutable BEFORE DELETE OR UPDATE ON public.project_metadata_changes FOR EACH ROW EXECUTE FUNCTION public.reject_project_metadata_change();

CREATE TRIGGER project_metadata_no_truncate BEFORE TRUNCATE ON public.project_metadata_changes FOR EACH STATEMENT EXECUTE FUNCTION public.reject_project_metadata_change();

CREATE TRIGGER project_responsibility_events_immutable BEFORE DELETE OR UPDATE ON public.project_responsibility_events FOR EACH ROW EXECUTE FUNCTION public.protect_project_responsibility_evidence();

CREATE TRIGGER project_responsibility_events_no_truncate BEFORE TRUNCATE ON public.project_responsibility_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_project_responsibility_evidence();

CREATE TRIGGER project_responsibility_no_delete BEFORE DELETE ON public.project_responsibility_profiles FOR EACH ROW EXECUTE FUNCTION public.protect_project_responsibility_evidence();

CREATE TRIGGER project_responsibility_no_truncate BEFORE TRUNCATE ON public.project_responsibility_profiles FOR EACH STATEMENT EXECUTE FUNCTION public.protect_project_responsibility_evidence();

CREATE TRIGGER project_responsibility_scope BEFORE UPDATE ON public.project_responsibility_profiles FOR EACH ROW EXECUTE FUNCTION public.protect_project_responsibility_scope();

CREATE TRIGGER project_schedule_history_immutable BEFORE DELETE OR UPDATE ON public.project_schedule_changes FOR EACH ROW EXECUTE FUNCTION public.reject_calendar_history_change();

CREATE TRIGGER project_schedule_history_no_truncate BEFORE TRUNCATE ON public.project_schedule_changes FOR EACH STATEMENT EXECUTE FUNCTION public.reject_calendar_history_change();

CREATE TRIGGER purchase_acceptance_event_immutable BEFORE DELETE OR UPDATE ON public.purchase_acceptance_events FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_acceptance_history();

CREATE CONSTRAINT TRIGGER purchase_acceptance_event_integrity AFTER INSERT ON public.purchase_acceptance_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_acceptance_event();

CREATE TRIGGER purchase_acceptance_event_no_truncate BEFORE TRUNCATE ON public.purchase_acceptance_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_acceptance_history();

CREATE TRIGGER purchase_acceptance_guard BEFORE INSERT OR UPDATE ON public.purchase_acceptances FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_acceptance();

CREATE TRIGGER purchase_acceptance_no_delete BEFORE DELETE ON public.purchase_acceptances FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_acceptance_history();

CREATE TRIGGER purchase_acceptance_no_truncate BEFORE TRUNCATE ON public.purchase_acceptances FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_acceptance_history();

CREATE CONSTRAINT TRIGGER purchase_acceptance_write_integrity AFTER INSERT OR UPDATE ON public.purchase_acceptances DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_acceptance_write();

CREATE TRIGGER purchase_advance_event_immutable BEFORE DELETE OR UPDATE ON public.purchase_advance_events FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_advance_history();

CREATE CONSTRAINT TRIGGER purchase_advance_event_integrity AFTER INSERT ON public.purchase_advance_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_advance_event();

CREATE TRIGGER purchase_advance_event_no_truncate BEFORE TRUNCATE ON public.purchase_advance_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_advance_history();

CREATE TRIGGER purchase_advance_guard BEFORE INSERT OR UPDATE ON public.purchase_advances FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_advance();

CREATE TRIGGER purchase_advance_no_delete BEFORE DELETE ON public.purchase_advances FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_advance_history();

CREATE TRIGGER purchase_advance_no_truncate BEFORE TRUNCATE ON public.purchase_advances FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_advance_history();

CREATE CONSTRAINT TRIGGER purchase_advance_write_integrity AFTER INSERT OR UPDATE ON public.purchase_advances DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_advance_write();

CREATE TRIGGER purchase_bo_events_immutable BEFORE DELETE OR UPDATE ON public.purchase_bo_events FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_bo_history();

CREATE TRIGGER purchase_bo_events_no_truncate BEFORE TRUNCATE ON public.purchase_bo_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_bo_history();

CREATE TRIGGER purchase_bo_line_owner BEFORE UPDATE ON public.purchase_bo_lines FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_bo_line_owner();

CREATE TRIGGER purchase_bo_lines_no_delete BEFORE DELETE ON public.purchase_bo_lines FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_bo_history();

CREATE TRIGGER purchase_bo_lines_no_truncate BEFORE TRUNCATE ON public.purchase_bo_lines FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_bo_history();

CREATE TRIGGER purchase_bo_no_delete BEFORE DELETE ON public.purchase_bo_lists FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_bo_history();

CREATE TRIGGER purchase_bo_no_truncate BEFORE TRUNCATE ON public.purchase_bo_lists FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_bo_history();

CREATE TRIGGER purchase_bo_ownership BEFORE UPDATE ON public.purchase_bo_lists FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_bo_ownership();

CREATE TRIGGER purchase_client_clearance_event_immutable BEFORE DELETE OR UPDATE ON public.purchase_client_clearance_events FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_client_clearance_history();

CREATE CONSTRAINT TRIGGER purchase_client_clearance_event_integrity AFTER INSERT ON public.purchase_client_clearance_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_client_clearance_event();

CREATE TRIGGER purchase_client_clearance_event_no_truncate BEFORE TRUNCATE ON public.purchase_client_clearance_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_client_clearance_history();

CREATE TRIGGER purchase_client_clearance_guard BEFORE INSERT OR UPDATE ON public.purchase_client_clearances FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_client_clearance();

CREATE TRIGGER purchase_client_clearance_no_delete BEFORE DELETE ON public.purchase_client_clearances FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_client_clearance_history();

CREATE TRIGGER purchase_client_clearance_no_truncate BEFORE TRUNCATE ON public.purchase_client_clearances FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_client_clearance_history();

CREATE CONSTRAINT TRIGGER purchase_client_clearance_write_integrity AFTER INSERT OR UPDATE ON public.purchase_client_clearances DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_client_clearance_write();

CREATE TRIGGER purchase_dispatch_event_immutable BEFORE DELETE OR UPDATE ON public.purchase_dispatch_events FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_dispatch_history();

CREATE CONSTRAINT TRIGGER purchase_dispatch_event_integrity AFTER INSERT ON public.purchase_dispatch_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_dispatch_event();

CREATE TRIGGER purchase_dispatch_event_no_truncate BEFORE TRUNCATE ON public.purchase_dispatch_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_dispatch_history();

CREATE TRIGGER purchase_dispatch_guard BEFORE INSERT OR UPDATE ON public.purchase_dispatches FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_dispatch();

CREATE TRIGGER purchase_dispatch_no_delete BEFORE DELETE ON public.purchase_dispatches FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_dispatch_history();

CREATE TRIGGER purchase_dispatch_no_truncate BEFORE TRUNCATE ON public.purchase_dispatches FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_dispatch_history();

CREATE CONSTRAINT TRIGGER purchase_dispatch_write_integrity AFTER INSERT OR UPDATE ON public.purchase_dispatches DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_dispatch_write();

CREATE TRIGGER purchase_engineer_confirmations_event_immutable BEFORE DELETE OR UPDATE ON public.purchase_engineer_confirmations_events FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_engineer_confirmations_history();

CREATE CONSTRAINT TRIGGER purchase_engineer_confirmations_event_integrity AFTER INSERT ON public.purchase_engineer_confirmations_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_engineer_confirmations_event();

CREATE TRIGGER purchase_engineer_confirmations_event_no_truncate BEFORE TRUNCATE ON public.purchase_engineer_confirmations_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_engineer_confirmations_history();

CREATE TRIGGER purchase_engineer_confirmations_guard BEFORE INSERT OR UPDATE ON public.purchase_engineer_confirmations FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_engineer_confirmations();

CREATE TRIGGER purchase_engineer_confirmations_no_delete BEFORE DELETE ON public.purchase_engineer_confirmations FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_engineer_confirmations_history();

CREATE TRIGGER purchase_engineer_confirmations_no_truncate BEFORE TRUNCATE ON public.purchase_engineer_confirmations FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_engineer_confirmations_history();

CREATE CONSTRAINT TRIGGER purchase_engineer_confirmations_write_integrity AFTER INSERT OR UPDATE ON public.purchase_engineer_confirmations DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_engineer_confirmations_write();

CREATE TRIGGER purchase_fms_history_immutable BEFORE DELETE OR UPDATE ON public.purchase_fms_owner_history FOR EACH ROW EXECUTE FUNCTION public.purchase_fms_history_immutable();

CREATE TRIGGER purchase_fms_history_no_truncate BEFORE TRUNCATE ON public.purchase_fms_owner_history FOR EACH STATEMENT EXECUTE FUNCTION public.purchase_fms_history_immutable();

CREATE TRIGGER purchase_grns_event_immutable BEFORE DELETE OR UPDATE ON public.purchase_grns_events FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_grns_history();

CREATE CONSTRAINT TRIGGER purchase_grns_event_integrity AFTER INSERT ON public.purchase_grns_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_grns_event();

CREATE TRIGGER purchase_grns_event_no_truncate BEFORE TRUNCATE ON public.purchase_grns_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_grns_history();

CREATE TRIGGER purchase_grns_guard BEFORE INSERT OR UPDATE ON public.purchase_grns FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_grns();

CREATE TRIGGER purchase_grns_no_delete BEFORE DELETE ON public.purchase_grns FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_grns_history();

CREATE TRIGGER purchase_grns_no_truncate BEFORE TRUNCATE ON public.purchase_grns FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_grns_history();

CREATE CONSTRAINT TRIGGER purchase_grns_write_integrity AFTER INSERT OR UPDATE ON public.purchase_grns DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_grns_write();

CREATE TRIGGER purchase_group_quotation_owner BEFORE INSERT OR UPDATE ON public.purchase_group_quotations FOR EACH ROW EXECUTE FUNCTION public.protect_quotation_owner();

CREATE TRIGGER purchase_group_quotations_no_delete BEFORE DELETE ON public.purchase_group_quotations FOR EACH ROW EXECUTE FUNCTION public.protect_quotation_evidence();

CREATE TRIGGER purchase_group_quotations_no_truncate BEFORE TRUNCATE ON public.purchase_group_quotations FOR EACH STATEMENT EXECUTE FUNCTION public.protect_quotation_evidence();

CREATE TRIGGER purchase_hold_events_immutable BEFORE DELETE OR UPDATE ON public.purchase_hold_events FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_hold_history();

CREATE TRIGGER purchase_hold_events_no_truncate BEFORE TRUNCATE ON public.purchase_hold_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_hold_history();

CREATE TRIGGER purchase_hold_scope BEFORE UPDATE ON public.purchase_holds FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_hold_scope();

CREATE TRIGGER purchase_holds_no_delete BEFORE DELETE ON public.purchase_holds FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_hold_history();

CREATE TRIGGER purchase_holds_no_truncate BEFORE TRUNCATE ON public.purchase_holds FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_hold_history();

CREATE TRIGGER purchase_indent_document_events_immutable BEFORE DELETE OR UPDATE ON public.purchase_indent_document_events FOR EACH ROW EXECUTE FUNCTION public.protect_indent_document_evidence();

CREATE TRIGGER purchase_indent_document_events_no_truncate BEFORE TRUNCATE ON public.purchase_indent_document_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_indent_document_evidence();

CREATE TRIGGER purchase_indent_document_scope BEFORE UPDATE ON public.purchase_indent_documents FOR EACH ROW EXECUTE FUNCTION public.protect_indent_document_scope();

CREATE TRIGGER purchase_indent_documents_no_delete BEFORE DELETE ON public.purchase_indent_documents FOR EACH ROW EXECUTE FUNCTION public.protect_indent_document_evidence();

CREATE TRIGGER purchase_indent_documents_no_truncate BEFORE TRUNCATE ON public.purchase_indent_documents FOR EACH STATEMENT EXECUTE FUNCTION public.protect_indent_document_evidence();

CREATE TRIGGER purchase_indent_events_immutable BEFORE DELETE OR UPDATE ON public.purchase_indent_events FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_indent_history();

CREATE TRIGGER purchase_indent_events_no_truncate BEFORE TRUNCATE ON public.purchase_indent_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_indent_history();

CREATE TRIGGER purchase_indent_group_entries_no_delete BEFORE DELETE ON public.purchase_indent_group_entries FOR EACH ROW EXECUTE FUNCTION public.protect_indent_group_evidence();

CREATE TRIGGER purchase_indent_group_entries_no_truncate BEFORE TRUNCATE ON public.purchase_indent_group_entries FOR EACH STATEMENT EXECUTE FUNCTION public.protect_indent_group_evidence();

CREATE TRIGGER purchase_indent_group_entry_owner BEFORE UPDATE ON public.purchase_indent_group_entries FOR EACH ROW EXECUTE FUNCTION public.protect_indent_group_owner();

CREATE TRIGGER purchase_indent_group_events_immutable BEFORE DELETE OR UPDATE ON public.purchase_indent_group_events FOR EACH ROW EXECUTE FUNCTION public.protect_indent_group_evidence();

CREATE TRIGGER purchase_indent_group_events_no_truncate BEFORE TRUNCATE ON public.purchase_indent_group_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_indent_group_evidence();

CREATE TRIGGER purchase_indent_group_owner BEFORE UPDATE ON public.purchase_indent_group_sets FOR EACH ROW EXECUTE FUNCTION public.protect_indent_group_owner();

CREATE TRIGGER purchase_indent_group_project_owner BEFORE UPDATE ON public.purchase_indent_group_sets FOR EACH ROW EXECUTE FUNCTION public.protect_indent_group_project();

CREATE TRIGGER purchase_indent_groups_no_delete BEFORE DELETE ON public.purchase_indent_group_sets FOR EACH ROW EXECUTE FUNCTION public.protect_indent_group_evidence();

CREATE TRIGGER purchase_indent_groups_no_truncate BEFORE TRUNCATE ON public.purchase_indent_group_sets FOR EACH STATEMENT EXECUTE FUNCTION public.protect_indent_group_evidence();

CREATE TRIGGER purchase_indent_legacy_terms_immutable BEFORE DELETE OR UPDATE ON public.purchase_indent_legacy_terms FOR EACH ROW EXECUTE FUNCTION public.protect_indent_document_evidence();

CREATE TRIGGER purchase_indent_legacy_terms_no_truncate BEFORE TRUNCATE ON public.purchase_indent_legacy_terms FOR EACH STATEMENT EXECUTE FUNCTION public.protect_indent_document_evidence();

CREATE TRIGGER purchase_indent_notice_owner BEFORE UPDATE ON public.purchase_indent_dispatch_notices FOR EACH ROW EXECUTE FUNCTION public.protect_indent_notice_owner();

CREATE TRIGGER purchase_indent_notices_no_delete BEFORE DELETE ON public.purchase_indent_dispatch_notices FOR EACH ROW EXECUTE FUNCTION public.protect_indent_document_evidence();

CREATE TRIGGER purchase_indent_notices_no_truncate BEFORE TRUNCATE ON public.purchase_indent_dispatch_notices FOR EACH STATEMENT EXECUTE FUNCTION public.protect_indent_document_evidence();

CREATE TRIGGER purchase_indent_release_immutable BEFORE DELETE OR UPDATE ON public.purchase_indent_releases FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_indent_history();

CREATE TRIGGER purchase_indent_release_no_truncate BEFORE TRUNCATE ON public.purchase_indent_releases FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_indent_history();

CREATE TRIGGER purchase_indent_scope BEFORE UPDATE ON public.purchase_indents FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_indent_scope();

CREATE TRIGGER purchase_indents_no_delete BEFORE DELETE ON public.purchase_indents FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_indent_history();

CREATE TRIGGER purchase_indents_no_truncate BEFORE TRUNCATE ON public.purchase_indents FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_indent_history();

CREATE TRIGGER purchase_inspection_event_immutable BEFORE DELETE OR UPDATE ON public.purchase_inspection_events FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_inspection_history();

CREATE CONSTRAINT TRIGGER purchase_inspection_event_integrity AFTER INSERT ON public.purchase_inspection_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_inspection_event();

CREATE TRIGGER purchase_inspection_event_no_truncate BEFORE TRUNCATE ON public.purchase_inspection_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_inspection_history();

CREATE TRIGGER purchase_inspection_guard BEFORE INSERT OR UPDATE ON public.purchase_inspections FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_inspection();

CREATE TRIGGER purchase_inspection_no_delete BEFORE DELETE ON public.purchase_inspections FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_inspection_history();

CREATE TRIGGER purchase_inspection_no_truncate BEFORE TRUNCATE ON public.purchase_inspections FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_inspection_history();

CREATE TRIGGER purchase_inspection_report_event_immutable BEFORE DELETE OR UPDATE ON public.purchase_inspection_report_events FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_inspection_report_history();

CREATE CONSTRAINT TRIGGER purchase_inspection_report_event_integrity AFTER INSERT ON public.purchase_inspection_report_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_inspection_report_event();

CREATE TRIGGER purchase_inspection_report_event_no_truncate BEFORE TRUNCATE ON public.purchase_inspection_report_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_inspection_report_history();

CREATE TRIGGER purchase_inspection_report_guard BEFORE INSERT OR UPDATE ON public.purchase_inspection_reports FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_inspection_report();

CREATE TRIGGER purchase_inspection_report_no_delete BEFORE DELETE ON public.purchase_inspection_reports FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_inspection_report_history();

CREATE TRIGGER purchase_inspection_report_no_truncate BEFORE TRUNCATE ON public.purchase_inspection_reports FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_inspection_report_history();

CREATE CONSTRAINT TRIGGER purchase_inspection_report_write_integrity AFTER INSERT OR UPDATE ON public.purchase_inspection_reports DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_inspection_report_write();

CREATE CONSTRAINT TRIGGER purchase_inspection_write_integrity AFTER INSERT OR UPDATE ON public.purchase_inspections DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_inspection_write();

CREATE CONSTRAINT TRIGGER purchase_loi_audit AFTER INSERT ON public.purchase_loi_document_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_loi_audit();

CREATE TRIGGER purchase_loi_events_immutable BEFORE DELETE OR UPDATE ON public.purchase_loi_document_events FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_sourcing_history();

CREATE TRIGGER purchase_loi_events_no_truncate BEFORE TRUNCATE ON public.purchase_loi_document_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_sourcing_history();

CREATE CONSTRAINT TRIGGER purchase_loi_generation AFTER INSERT ON public.purchase_loi_documents DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_loi_generation();

CREATE TRIGGER purchase_loi_handoff BEFORE INSERT ON public.purchase_loi_documents FOR EACH ROW EXECUTE FUNCTION public.check_purchase_loi_handoff();

CREATE TRIGGER purchase_loi_immutable BEFORE DELETE OR UPDATE ON public.purchase_loi_documents FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_sourcing_history();

CREATE TRIGGER purchase_loi_no_truncate BEFORE TRUNCATE ON public.purchase_loi_documents FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_sourcing_history();

CREATE CONSTRAINT TRIGGER purchase_loi_review_audit AFTER INSERT ON public.purchase_loi_reviews DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_loi_review_audit();

CREATE TRIGGER purchase_loi_review_immutable BEFORE DELETE OR UPDATE ON public.purchase_loi_reviews FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_sourcing_history();

CREATE TRIGGER purchase_loi_review_no_truncate BEFORE TRUNCATE ON public.purchase_loi_reviews FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_sourcing_history();

CREATE TRIGGER purchase_order_document_immutable BEFORE DELETE OR UPDATE ON public.purchase_order_document_events FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_order_history();

CREATE CONSTRAINT TRIGGER purchase_order_document_integrity AFTER INSERT ON public.purchase_order_document_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_order_document();

CREATE TRIGGER purchase_order_document_no_truncate BEFORE TRUNCATE ON public.purchase_order_document_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_order_history();

CREATE TRIGGER purchase_order_event_immutable BEFORE DELETE OR UPDATE ON public.purchase_order_events FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_order_history();

CREATE CONSTRAINT TRIGGER purchase_order_event_integrity AFTER INSERT ON public.purchase_order_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_order_event();

CREATE TRIGGER purchase_order_event_no_truncate BEFORE TRUNCATE ON public.purchase_order_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_order_history();

CREATE TRIGGER purchase_order_guard BEFORE INSERT OR UPDATE ON public.purchase_orders FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_order();

CREATE TRIGGER purchase_order_no_delete BEFORE DELETE ON public.purchase_orders FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_order_history();

CREATE TRIGGER purchase_order_no_truncate BEFORE TRUNCATE ON public.purchase_orders FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_order_history();

CREATE CONSTRAINT TRIGGER purchase_order_write_integrity AFTER INSERT OR UPDATE ON public.purchase_orders DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_order_write();

CREATE TRIGGER purchase_quality_case_guard BEFORE INSERT OR UPDATE ON public.purchase_quality_cases FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_quality_case();

CREATE TRIGGER purchase_quality_case_no_delete BEFORE DELETE ON public.purchase_quality_cases FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_quality_history();

CREATE TRIGGER purchase_quality_case_no_truncate BEFORE TRUNCATE ON public.purchase_quality_cases FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_quality_history();

CREATE TRIGGER purchase_quality_certificate_guard BEFORE INSERT OR UPDATE ON public.purchase_quality_certificates FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_quality_certificate();

CREATE TRIGGER purchase_quality_certificate_no_delete BEFORE DELETE ON public.purchase_quality_certificates FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_quality_history();

CREATE TRIGGER purchase_quality_certificate_no_truncate BEFORE TRUNCATE ON public.purchase_quality_certificates FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_quality_history();

CREATE CONSTRAINT TRIGGER purchase_quality_child_integrity AFTER INSERT OR UPDATE ON public.purchase_quality_certificates DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_quality_children();

CREATE CONSTRAINT TRIGGER purchase_quality_children_integrity AFTER INSERT OR UPDATE ON public.purchase_quality_cases DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_quality_children();

CREATE TRIGGER purchase_quality_event_immutable BEFORE DELETE OR UPDATE ON public.purchase_quality_events FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_quality_history();

CREATE CONSTRAINT TRIGGER purchase_quality_event_integrity AFTER INSERT ON public.purchase_quality_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_quality_event();

CREATE TRIGGER purchase_quality_event_no_truncate BEFORE TRUNCATE ON public.purchase_quality_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_quality_history();

CREATE CONSTRAINT TRIGGER purchase_quality_write_integrity AFTER INSERT OR UPDATE ON public.purchase_quality_cases DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_quality_write();

CREATE TRIGGER purchase_quotation_attachments_immutable BEFORE DELETE OR UPDATE ON public.purchase_quotation_attachments FOR EACH ROW EXECUTE FUNCTION public.protect_quotation_evidence();

CREATE TRIGGER purchase_quotation_attachments_no_truncate BEFORE TRUNCATE ON public.purchase_quotation_attachments FOR EACH STATEMENT EXECUTE FUNCTION public.protect_quotation_evidence();

CREATE TRIGGER purchase_quotation_events_immutable BEFORE DELETE OR UPDATE ON public.purchase_quotation_events FOR EACH ROW EXECUTE FUNCTION public.protect_quotation_evidence();

CREATE TRIGGER purchase_quotation_events_no_truncate BEFORE TRUNCATE ON public.purchase_quotation_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_quotation_evidence();

CREATE TRIGGER purchase_quotation_evidence_events_immutable BEFORE DELETE OR UPDATE ON public.purchase_quotation_evidence_events FOR EACH ROW EXECUTE FUNCTION public.protect_quotation_evidence();

CREATE TRIGGER purchase_quotation_evidence_events_no_truncate BEFORE TRUNCATE ON public.purchase_quotation_evidence_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_quotation_evidence();

CREATE TRIGGER purchase_quotation_revision_scope BEFORE INSERT ON public.purchase_quotation_revisions FOR EACH ROW EXECUTE FUNCTION public.protect_quotation_revision_scope();

CREATE TRIGGER purchase_quotation_revisions_immutable BEFORE DELETE OR UPDATE ON public.purchase_quotation_revisions FOR EACH ROW EXECUTE FUNCTION public.protect_quotation_evidence();

CREATE TRIGGER purchase_quotation_revisions_no_truncate BEFORE TRUNCATE ON public.purchase_quotation_revisions FOR EACH STATEMENT EXECUTE FUNCTION public.protect_quotation_evidence();

CREATE TRIGGER purchase_quotation_shortlist_history_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON public.purchase_quotation_shortlist_history FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER purchase_quotation_shortlist_no_delete BEFORE DELETE OR TRUNCATE ON public.purchase_quotation_shortlist_drafts FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER purchase_quotation_work_no_delete BEFORE DELETE OR TRUNCATE ON public.purchase_quotation_work_tasks FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER purchase_site_receipts_event_immutable BEFORE DELETE OR UPDATE ON public.purchase_site_receipts_events FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_site_receipts_history();

CREATE CONSTRAINT TRIGGER purchase_site_receipts_event_integrity AFTER INSERT ON public.purchase_site_receipts_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_site_receipts_event();

CREATE TRIGGER purchase_site_receipts_event_no_truncate BEFORE TRUNCATE ON public.purchase_site_receipts_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_site_receipts_history();

CREATE TRIGGER purchase_site_receipts_guard BEFORE INSERT OR UPDATE ON public.purchase_site_receipts FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_site_receipts();

CREATE TRIGGER purchase_site_receipts_no_delete BEFORE DELETE ON public.purchase_site_receipts FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_site_receipts_history();

CREATE TRIGGER purchase_site_receipts_no_truncate BEFORE TRUNCATE ON public.purchase_site_receipts FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_site_receipts_history();

CREATE CONSTRAINT TRIGGER purchase_site_receipts_write_integrity AFTER INSERT OR UPDATE ON public.purchase_site_receipts DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_site_receipts_write();

CREATE TRIGGER purchase_sourcing_allocation_guard BEFORE INSERT OR UPDATE ON public.purchase_sourcing_allocations FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_sourcing_allocation();

CREATE CONSTRAINT TRIGGER purchase_sourcing_allocation_integrity AFTER INSERT OR UPDATE ON public.purchase_sourcing_allocations DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_sourcing_allocation();

CREATE TRIGGER purchase_sourcing_allocation_no_delete BEFORE DELETE ON public.purchase_sourcing_allocations FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_sourcing_history();

CREATE TRIGGER purchase_sourcing_allocation_no_truncate BEFORE TRUNCATE ON public.purchase_sourcing_allocations FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_sourcing_history();

CREATE TRIGGER purchase_sourcing_event_immutable BEFORE DELETE OR UPDATE ON public.purchase_sourcing_events FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_sourcing_history();

CREATE CONSTRAINT TRIGGER purchase_sourcing_event_integrity AFTER INSERT ON public.purchase_sourcing_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_sourcing_event();

CREATE TRIGGER purchase_sourcing_event_no_truncate BEFORE TRUNCATE ON public.purchase_sourcing_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_sourcing_history();

CREATE TRIGGER purchase_sourcing_no_delete BEFORE DELETE ON public.purchase_sourcing_cases FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_sourcing_history();

CREATE TRIGGER purchase_sourcing_no_truncate BEFORE TRUNCATE ON public.purchase_sourcing_cases FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_sourcing_history();

CREATE CONSTRAINT TRIGGER purchase_sourcing_reservation_integrity AFTER INSERT OR UPDATE ON public.purchase_sourcing_cases DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_sourcing_reservation();

CREATE TRIGGER purchase_sourcing_scope BEFORE INSERT OR UPDATE ON public.purchase_sourcing_cases FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_sourcing();

CREATE CONSTRAINT TRIGGER purchase_sourcing_write_integrity AFTER INSERT OR UPDATE ON public.purchase_sourcing_cases DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_sourcing_write();

CREATE TRIGGER purchase_technical_event_immutable BEFORE DELETE OR UPDATE ON public.purchase_technical_events FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_technical_history();

CREATE CONSTRAINT TRIGGER purchase_technical_event_integrity AFTER INSERT ON public.purchase_technical_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_technical_event();

CREATE TRIGGER purchase_technical_event_no_truncate BEFORE TRUNCATE ON public.purchase_technical_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_technical_history();

CREATE TRIGGER purchase_technical_file_event_immutable BEFORE DELETE OR UPDATE ON public.purchase_technical_file_events FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_technical_history();

CREATE CONSTRAINT TRIGGER purchase_technical_file_event_integrity AFTER INSERT ON public.purchase_technical_file_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_technical_file_event();

CREATE TRIGGER purchase_technical_file_event_no_truncate BEFORE TRUNCATE ON public.purchase_technical_file_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_technical_history();

CREATE TRIGGER purchase_technical_file_immutable BEFORE DELETE OR UPDATE ON public.purchase_technical_files FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_technical_history();

CREATE CONSTRAINT TRIGGER purchase_technical_file_integrity AFTER INSERT ON public.purchase_technical_files DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_technical_file();

CREATE TRIGGER purchase_technical_file_no_truncate BEFORE TRUNCATE ON public.purchase_technical_files FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_technical_history();

CREATE TRIGGER purchase_technical_guard BEFORE INSERT OR UPDATE ON public.purchase_technical_cases FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_technical();

CREATE TRIGGER purchase_technical_no_delete BEFORE DELETE ON public.purchase_technical_cases FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_technical_history();

CREATE TRIGGER purchase_technical_no_truncate BEFORE TRUNCATE ON public.purchase_technical_cases FOR EACH STATEMENT EXECUTE FUNCTION public.protect_purchase_technical_history();

CREATE CONSTRAINT TRIGGER purchase_technical_write_integrity AFTER INSERT OR UPDATE ON public.purchase_technical_cases DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_purchase_technical_write();

CREATE TRIGGER purchase_work_events_immutable BEFORE DELETE OR UPDATE ON public.purchase_work_package_events FOR EACH ROW EXECUTE FUNCTION public.refuse_purchase_work_evidence_mutation();

CREATE TRIGGER purchase_work_events_no_truncate BEFORE TRUNCATE ON public.purchase_work_package_events FOR EACH STATEMENT EXECUTE FUNCTION public.refuse_purchase_work_evidence_mutation();

CREATE TRIGGER purchase_work_package_guard BEFORE INSERT OR UPDATE ON public.purchase_work_packages FOR EACH ROW EXECUTE FUNCTION public.protect_purchase_work_package();

CREATE TRIGGER purchase_work_packages_no_delete BEFORE DELETE ON public.purchase_work_packages FOR EACH ROW EXECUTE FUNCTION public.refuse_purchase_work_evidence_mutation();

CREATE TRIGGER purchase_work_packages_no_truncate BEFORE TRUNCATE ON public.purchase_work_packages FOR EACH STATEMENT EXECUTE FUNCTION public.refuse_purchase_work_evidence_mutation();

CREATE CONSTRAINT TRIGGER quotation_approval_finance AFTER INSERT OR UPDATE ON public.quotation_approval_sets DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_quotation_approval_decisions();

CREATE TRIGGER quotation_approval_history_immutable BEFORE DELETE OR UPDATE ON public.quotation_approval_events FOR EACH ROW EXECUTE FUNCTION public.protect_quotation_approval_history();

CREATE TRIGGER quotation_approval_history_no_truncate BEFORE TRUNCATE ON public.quotation_approval_events FOR EACH STATEMENT EXECUTE FUNCTION public.protect_quotation_approval_history();

CREATE TRIGGER quotation_approval_no_delete BEFORE DELETE ON public.quotation_approval_sets FOR EACH ROW EXECUTE FUNCTION public.protect_quotation_approval_history();

CREATE TRIGGER quotation_approval_no_truncate BEFORE TRUNCATE ON public.quotation_approval_sets FOR EACH STATEMENT EXECUTE FUNCTION public.protect_quotation_approval_history();

CREATE TRIGGER quotation_approval_scope BEFORE INSERT OR UPDATE ON public.quotation_approval_sets FOR EACH ROW EXECUTE FUNCTION public.protect_quotation_approval_scope();

CREATE TRIGGER quotation_shortlist_sources BEFORE INSERT OR UPDATE ON public.quotation_approval_sets FOR EACH ROW EXECUTE FUNCTION public.check_quotation_shortlist_sources();

CREATE TRIGGER supplier_changes_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON public.supplier_changes FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER supplier_document_approval_changes_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON public.supplier_document_approval_changes FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER supplier_document_changes_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON public.supplier_document_changes FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER supplier_document_completed_task_immutable BEFORE DELETE OR UPDATE ON public.supplier_document_tasks FOR EACH ROW EXECUTE FUNCTION public.epcerp_preserve_completed_document_task();

CREATE TRIGGER supplier_review_assignment_history_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON public.supplier_review_assignment_changes FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER supplier_verification_history_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON public.supplier_verification_history FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER cloud_reservation_immutable BEFORE DELETE OR UPDATE ON tender.tsc_cloud_calls FOR EACH ROW EXECUTE FUNCTION tender.protect_cloud_reservation();

CREATE TRIGGER cloud_reservation_no_truncate BEFORE TRUNCATE ON tender.tsc_cloud_calls FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER draft_document_removals_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON tender.draft_document_removals FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER draft_events_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON tender.draft_events FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER draft_sources_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON tender.draft_sources FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER immutable_tender_intake_event BEFORE DELETE OR UPDATE ON tender.tender_fms_events FOR EACH ROW EXECUTE FUNCTION public.protect_reading_evidence();

CREATE TRIGGER tender_capture_operations_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON tender.tender_capture_operations FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER tender_email_attempts_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON tender.tender_email_attempts FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER tender_fms_manager_assignments_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON tender.tender_fms_manager_assignments FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER tender_fms_owner_changes_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON tender.tender_fms_owner_changes FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER tender_fms_participant_grants_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON tender.tender_fms_participant_grants FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER tender_fms_record_events_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON tender.tender_fms_record_events FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER tender_source_links_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON tender.tender_source_links FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER tsc_document_text_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON tender.tsc_document_text FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER tsc_events_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON tender.tsc_events FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER tsc_format_events_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON tender.tsc_format_events FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER tsc_format_snapshot_immutable BEFORE UPDATE ON tender.tsc_jobs FOR EACH ROW EXECUTE FUNCTION tender.protect_tsc_format_snapshot();

CREATE TRIGGER tsc_inputs_immutable BEFORE DELETE OR UPDATE ON tender.tsc_jobs FOR EACH ROW EXECUTE FUNCTION tender.protect_tsc_inputs();

CREATE TRIGGER tsc_jobs_no_truncate BEFORE TRUNCATE ON tender.tsc_jobs FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER tsc_model_call_immutable BEFORE DELETE OR UPDATE ON tender.tsc_model_calls FOR EACH ROW EXECUTE FUNCTION tender.protect_tsc_model_call();

CREATE TRIGGER tsc_model_call_no_truncate BEFORE TRUNCATE ON tender.tsc_model_calls FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER tsc_model_responses_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON tender.tsc_model_responses FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER tsc_task_immutable BEFORE DELETE OR UPDATE ON tender.tsc_workspace_tasks FOR EACH ROW EXECUTE FUNCTION tender.protect_tsc_workspace_task();

CREATE TRIGGER tsc_task_no_truncate BEFORE TRUNCATE ON tender.tsc_workspace_tasks FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER tsc_workspace_events_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON tender.tsc_workspace_events FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

CREATE TRIGGER tsc_workspace_immutable BEFORE DELETE OR UPDATE ON tender.tsc_workspaces FOR EACH ROW EXECUTE FUNCTION tender.protect_tsc_workspace();

CREATE TRIGGER tsc_workspace_no_truncate BEFORE TRUNCATE ON tender.tsc_workspaces FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();

ALTER TABLE ONLY auth.access_sessions
    ADD CONSTRAINT access_sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.administration_changes
    ADD CONSTRAINT administration_changes_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_bootstrap_identity
    ADD CONSTRAINT auth_bootstrap_identity_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_credential_changes
    ADD CONSTRAINT auth_credential_changes_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_credential_changes
    ADD CONSTRAINT auth_credential_changes_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_credential_emails
    ADD CONSTRAINT auth_credential_emails_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_credential_emails
    ADD CONSTRAINT auth_credential_emails_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_credential_emails
    ADD CONSTRAINT auth_credential_emails_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_departments
    ADD CONSTRAINT auth_departments_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_departments
    ADD CONSTRAINT auth_departments_manager_id_fkey FOREIGN KEY (manager_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_departments
    ADD CONSTRAINT auth_departments_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_offboarding_cases
    ADD CONSTRAINT auth_offboarding_cases_approval_by_id_fkey FOREIGN KEY (approval_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_offboarding_cases
    ADD CONSTRAINT auth_offboarding_cases_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_offboarding_cases
    ADD CONSTRAINT auth_offboarding_cases_manager_id_fkey FOREIGN KEY (manager_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_offboarding_cases
    ADD CONSTRAINT auth_offboarding_cases_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_offboarding_cases
    ADD CONSTRAINT auth_offboarding_cases_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_offboarding_changes
    ADD CONSTRAINT auth_offboarding_changes_case_id_fkey FOREIGN KEY (case_id) REFERENCES auth.auth_offboarding_cases(id);

ALTER TABLE ONLY auth.auth_offboarding_changes
    ADD CONSTRAINT auth_offboarding_changes_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_organisation_changes
    ADD CONSTRAINT auth_organisation_changes_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_password_recovery
    ADD CONSTRAINT auth_password_recovery_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_personal_contact_changes
    ADD CONSTRAINT auth_personal_contact_changes_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_personal_contact_changes
    ADD CONSTRAINT auth_personal_contact_changes_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_reporting_lines
    ADD CONSTRAINT auth_reporting_lines_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_reporting_lines
    ADD CONSTRAINT auth_reporting_lines_department_id_fkey FOREIGN KEY (department_id) REFERENCES auth.auth_departments(id);

ALTER TABLE ONLY auth.auth_reporting_lines
    ADD CONSTRAINT auth_reporting_lines_manager_id_fkey FOREIGN KEY (manager_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_reporting_lines
    ADD CONSTRAINT auth_reporting_lines_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_reporting_lines
    ADD CONSTRAINT auth_reporting_lines_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_user_status_changes
    ADD CONSTRAINT auth_user_status_changes_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_user_status_changes
    ADD CONSTRAINT auth_user_status_changes_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_username_changes
    ADD CONSTRAINT auth_username_changes_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.auth_username_changes
    ADD CONSTRAINT auth_username_changes_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.email_changes
    ADD CONSTRAINT email_changes_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.email_changes
    ADD CONSTRAINT email_changes_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.refresh_tokens
    ADD CONSTRAINT refresh_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY auth.role_permissions
    ADD CONSTRAINT role_permissions_permission_id_fkey FOREIGN KEY (permission_id) REFERENCES auth.permissions(id) ON DELETE CASCADE;

ALTER TABLE ONLY auth.role_permissions
    ADD CONSTRAINT role_permissions_role_id_fkey FOREIGN KEY (role_id) REFERENCES auth.roles(id) ON DELETE CASCADE;

ALTER TABLE ONLY auth.roles
    ADD CONSTRAINT roles_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY auth.roles
    ADD CONSTRAINT roles_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY auth.user_activity
    ADD CONSTRAINT user_activity_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY auth.user_profile
    ADD CONSTRAINT user_profiles_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.user_profile
    ADD CONSTRAINT user_profiles_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.user_profile
    ADD CONSTRAINT user_profiles_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id);

ALTER TABLE ONLY auth.user_roles
    ADD CONSTRAINT user_roles_role_id_fkey FOREIGN KEY (role_id) REFERENCES auth.roles(id) ON DELETE CASCADE;

ALTER TABLE ONLY auth.user_roles
    ADD CONSTRAINT user_roles_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY auth.users
    ADD CONSTRAINT users_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY auth.users
    ADD CONSTRAINT users_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE public.activity_events
    ADD CONSTRAINT activity_events_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES auth.users(id);

ALTER TABLE public.activity_events
    ADD CONSTRAINT activity_events_source_event_id_fkey FOREIGN KEY (source_event_id) REFERENCES public.business_events(id);

ALTER TABLE ONLY public.application_purchase_settings
    ADD CONSTRAINT application_purchase_settings_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.business_events
    ADD CONSTRAINT business_events_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.catalogue_item_events
    ADD CONSTRAINT catalogue_item_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.catalogue_item_events
    ADD CONSTRAINT catalogue_item_events_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES public.catalogue_items(id);

ALTER TABLE ONLY public.catalogue_items
    ADD CONSTRAINT catalogue_items_category_id_fkey FOREIGN KEY (category_id) REFERENCES public.categories(id);

ALTER TABLE ONLY public.catalogue_items
    ADD CONSTRAINT catalogue_items_configuration_id_fkey FOREIGN KEY (configuration_id) REFERENCES public.configuration_templates(id);

ALTER TABLE ONLY public.catalogue_items
    ADD CONSTRAINT catalogue_items_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.catalogue_items
    ADD CONSTRAINT catalogue_items_manufacturer_id_fkey FOREIGN KEY (manufacturer_id) REFERENCES public.manufacturers(id);

ALTER TABLE ONLY public.catalogue_items
    ADD CONSTRAINT catalogue_items_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.catalogue_units
    ADD CONSTRAINT catalogue_units_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.catalogue_units
    ADD CONSTRAINT catalogue_units_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.categories
    ADD CONSTRAINT categories_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.categories
    ADD CONSTRAINT categories_unit_id_fkey FOREIGN KEY (unit_id) REFERENCES public.catalogue_units(id);

ALTER TABLE ONLY public.categories
    ADD CONSTRAINT categories_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.category_catalogue_changes
    ADD CONSTRAINT category_catalogue_changes_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.category_images
    ADD CONSTRAINT category_images_category_id_fkey FOREIGN KEY (category_id) REFERENCES public.categories(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.category_images
    ADD CONSTRAINT category_images_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.category_subcategories
    ADD CONSTRAINT category_subcategories_category_id_fkey FOREIGN KEY (category_id) REFERENCES public.categories(id);

ALTER TABLE ONLY public.category_subcategories
    ADD CONSTRAINT category_subcategories_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.category_subcategories
    ADD CONSTRAINT category_subcategories_unit_id_fkey FOREIGN KEY (unit_id) REFERENCES public.catalogue_units(id);

ALTER TABLE ONLY public.category_subcategories
    ADD CONSTRAINT category_subcategories_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.client_contacts
    ADD CONSTRAINT client_contacts_client_id_fkey FOREIGN KEY (client_id) REFERENCES public.clients(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.client_contacts
    ADD CONSTRAINT client_contacts_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.client_contacts
    ADD CONSTRAINT client_contacts_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.client_purchase_rule_events
    ADD CONSTRAINT client_purchase_rule_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.client_purchase_rule_events
    ADD CONSTRAINT client_purchase_rule_events_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES public.client_purchase_rules(id);

ALTER TABLE ONLY public.client_purchase_rules
    ADD CONSTRAINT client_purchase_rules_client_id_fkey FOREIGN KEY (client_id) REFERENCES public.clients(id);

ALTER TABLE ONLY public.client_purchase_rules
    ADD CONSTRAINT client_purchase_rules_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.client_purchase_rules
    ADD CONSTRAINT client_purchase_rules_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.client_purchase_rules
    ADD CONSTRAINT client_purchase_rules_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.client_types
    ADD CONSTRAINT client_types_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.client_types
    ADD CONSTRAINT client_types_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.clients
    ADD CONSTRAINT clients_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.clients
    ADD CONSTRAINT clients_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.communication_notice_events
    ADD CONSTRAINT communication_notice_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.communication_notice_events
    ADD CONSTRAINT communication_notice_events_notice_id_fkey FOREIGN KEY (notice_id) REFERENCES public.communication_notices(id);

ALTER TABLE ONLY public.communication_notices
    ADD CONSTRAINT communication_notices_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.communication_notices
    ADD CONSTRAINT communication_notices_director_designation_id_fkey FOREIGN KEY (director_designation_id) REFERENCES public.organisation_director_designation(id);

ALTER TABLE ONLY public.communication_notices
    ADD CONSTRAINT communication_notices_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.communication_notices
    ADD CONSTRAINT communication_notices_recipient_id_fkey FOREIGN KEY (recipient_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.communication_notices
    ADD CONSTRAINT communication_notices_source_event_id_fkey FOREIGN KEY (source_event_id) REFERENCES public.business_events(id);

ALTER TABLE ONLY public.communication_notices
    ADD CONSTRAINT communication_notices_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.communication_notices
    ADD CONSTRAINT communication_tender_action_source FOREIGN KEY (source_action_id, source_event_id, recipient_id, source_version) REFERENCES tender.tender_source_actions(id, source_event_id, recipient_id, assignment_version);

ALTER TABLE ONLY public.company_certificates
    ADD CONSTRAINT company_certificates_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.company_financial_years
    ADD CONSTRAINT company_financial_years_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.company_licences
    ADD CONSTRAINT company_licences_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.company_profile
    ADD CONSTRAINT company_profile_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.company_work_experience
    ADD CONSTRAINT company_work_experience_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.configuration_templates
    ADD CONSTRAINT configuration_templates_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.configuration_templates
    ADD CONSTRAINT configuration_templates_subcategory_id_fkey FOREIGN KEY (subcategory_id) REFERENCES public.category_subcategories(id);

ALTER TABLE ONLY public.configuration_templates
    ADD CONSTRAINT configuration_templates_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.document_package_events
    ADD CONSTRAINT document_package_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.document_package_events
    ADD CONSTRAINT document_package_events_file_id_fkey FOREIGN KEY (file_id) REFERENCES public.document_package_files(id);

ALTER TABLE ONLY public.document_package_events
    ADD CONSTRAINT document_package_events_receipt_id_fkey FOREIGN KEY (receipt_id) REFERENCES public.document_package_receipts(id);

ALTER TABLE ONLY public.document_package_files
    ADD CONSTRAINT document_package_files_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.document_package_files
    ADD CONSTRAINT document_package_files_receipt_id_fkey FOREIGN KEY (receipt_id) REFERENCES public.document_package_receipts(id);

ALTER TABLE ONLY public.document_package_files
    ADD CONSTRAINT document_package_files_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.document_package_manifest
    ADD CONSTRAINT document_package_manifest_file_id_fkey FOREIGN KEY (file_id) REFERENCES public.document_package_files(id);

ALTER TABLE ONLY public.document_package_manifest
    ADD CONSTRAINT document_package_manifest_receipt_id_fkey FOREIGN KEY (receipt_id) REFERENCES public.document_package_receipts(id);

ALTER TABLE ONLY public.document_package_members
    ADD CONSTRAINT document_package_members_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.document_package_members
    ADD CONSTRAINT document_package_members_receipt_id_file_id_fkey FOREIGN KEY (receipt_id, file_id) REFERENCES public.document_package_files(receipt_id, id);

ALTER TABLE ONLY public.document_package_members
    ADD CONSTRAINT document_package_members_receipt_id_fkey FOREIGN KEY (receipt_id) REFERENCES public.document_package_receipts(id);

ALTER TABLE ONLY public.document_package_objects
    ADD CONSTRAINT document_package_objects_file_id_fkey FOREIGN KEY (file_id) REFERENCES public.document_package_files(id);

ALTER TABLE ONLY public.document_package_objects
    ADD CONSTRAINT document_package_objects_receipt_id_fkey FOREIGN KEY (receipt_id) REFERENCES public.document_package_receipts(id);

ALTER TABLE ONLY public.document_package_operations
    ADD CONSTRAINT document_package_operations_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.document_package_operations
    ADD CONSTRAINT document_package_operations_receipt_id_fkey FOREIGN KEY (receipt_id) REFERENCES public.document_package_receipts(id);

ALTER TABLE ONLY public.document_package_receipts
    ADD CONSTRAINT document_package_receipts_consumed_by_tender_id_fkey FOREIGN KEY (consumed_by_tender_id) REFERENCES tender.tender_fms_intakes(id);

ALTER TABLE ONLY public.document_package_receipts
    ADD CONSTRAINT document_package_receipts_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.document_package_receipts
    ADD CONSTRAINT document_package_receipts_draft_id_fkey FOREIGN KEY (draft_id) REFERENCES tender.document_drafts(id);

ALTER TABLE ONLY public.document_package_receipts
    ADD CONSTRAINT document_package_receipts_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.document_package_receipts
    ADD CONSTRAINT document_package_receipts_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.document_reading_events
    ADD CONSTRAINT document_reading_events_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.document_reading_events
    ADD CONSTRAINT document_reading_events_document_id_fkey FOREIGN KEY (document_id) REFERENCES public.document_readings(id);

ALTER TABLE ONLY public.document_reading_results
    ADD CONSTRAINT document_reading_results_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.document_reading_results
    ADD CONSTRAINT document_reading_results_document_id_fkey FOREIGN KEY (document_id) REFERENCES public.document_readings(id);

ALTER TABLE ONLY public.document_reading_results
    ADD CONSTRAINT document_reading_results_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.document_readings
    ADD CONSTRAINT document_readings_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.document_readings
    ADD CONSTRAINT document_readings_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.finance_accounts_acceptance
    ADD CONSTRAINT finance_accounts_acceptance_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.finance_accounts_acceptance
    ADD CONSTRAINT finance_accounts_acceptance_engineer_id_fkey FOREIGN KEY (engineer_id) REFERENCES public.purchase_engineer_confirmations(id);

ALTER TABLE ONLY public.finance_accounts_acceptance_events
    ADD CONSTRAINT finance_accounts_acceptance_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.finance_accounts_acceptance_events
    ADD CONSTRAINT finance_accounts_acceptance_events_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES public.finance_accounts_acceptance(id);

ALTER TABLE ONLY public.finance_accounts_acceptance
    ADD CONSTRAINT finance_accounts_acceptance_package_id_fkey FOREIGN KEY (package_id) REFERENCES public.purchase_work_packages(id);

ALTER TABLE ONLY public.finance_accounts_acceptance
    ADD CONSTRAINT finance_accounts_acceptance_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.finance_accounts_acceptance
    ADD CONSTRAINT finance_accounts_acceptance_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.finance_advance_decisions
    ADD CONSTRAINT finance_advance_decisions_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.finance_advance_decisions
    ADD CONSTRAINT finance_advance_decisions_id_fkey FOREIGN KEY (id) REFERENCES public.business_events(id) DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE ONLY public.finance_advance_decisions
    ADD CONSTRAINT finance_advance_decisions_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.purchase_orders(id);

ALTER TABLE ONLY public.finance_advance_decisions
    ADD CONSTRAINT finance_advance_decisions_request_id_project_id_package_id_fkey FOREIGN KEY (request_id, project_id, package_id) REFERENCES public.purchase_advances(id, project_id, package_id);

ALTER TABLE ONLY public.finance_advance_decisions
    ADD CONSTRAINT finance_advance_decisions_submitted_by_id_fkey FOREIGN KEY (submitted_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.finance_advance_decisions
    ADD CONSTRAINT finance_advance_decisions_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.finance_budget_decisions
    ADD CONSTRAINT finance_budget_decisions_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.finance_budget_decisions
    ADD CONSTRAINT finance_budget_decisions_id_fkey FOREIGN KEY (id) REFERENCES public.business_events(id) DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE ONLY public.finance_budget_decisions
    ADD CONSTRAINT finance_budget_decisions_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.finance_budget_decisions
    ADD CONSTRAINT finance_budget_decisions_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.finance_quotation_decisions
    ADD CONSTRAINT finance_quotation_decisions_bo_id_project_id_fkey FOREIGN KEY (bo_id, project_id) REFERENCES public.quotation_approval_sets(bo_id, project_id);

ALTER TABLE ONLY public.finance_quotation_decisions
    ADD CONSTRAINT finance_quotation_decisions_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.finance_quotation_decisions
    ADD CONSTRAINT finance_quotation_decisions_id_fkey FOREIGN KEY (id) REFERENCES public.business_events(id) DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE ONLY public.finance_quotation_decisions
    ADD CONSTRAINT finance_quotation_decisions_submitted_by_id_fkey FOREIGN KEY (submitted_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.finance_quotation_decisions
    ADD CONSTRAINT finance_quotation_decisions_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.indian_cities
    ADD CONSTRAINT indian_cities_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.indian_cities
    ADD CONSTRAINT indian_cities_state_id_fkey FOREIGN KEY (state_id) REFERENCES public.indian_states(id) ON DELETE RESTRICT;

ALTER TABLE ONLY public.indian_cities
    ADD CONSTRAINT indian_cities_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.indian_states
    ADD CONSTRAINT indian_states_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.indian_states
    ADD CONSTRAINT indian_states_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.industries
    ADD CONSTRAINT industries_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.industries
    ADD CONSTRAINT industries_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.manufacturer_imports
    ADD CONSTRAINT manufacturer_imports_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.manufacturer_imports
    ADD CONSTRAINT manufacturer_imports_go_id_fkey FOREIGN KEY (go_id) REFERENCES public.manufacturers(id);

ALTER TABLE ONLY public.manufacturer_imports
    ADD CONSTRAINT manufacturer_imports_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.manufacturers
    ADD CONSTRAINT manufacturers_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.manufacturers_events
    ADD CONSTRAINT manufacturers_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.manufacturers_events
    ADD CONSTRAINT manufacturers_events_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES public.manufacturers(id);

ALTER TABLE ONLY public.manufacturers
    ADD CONSTRAINT manufacturers_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.material_catalogue
    ADD CONSTRAINT material_catalogue_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.material_catalogue
    ADD CONSTRAINT material_catalogue_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.organisation_director_designation
    ADD CONSTRAINT organisation_director_designation_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.organisation_director_designation
    ADD CONSTRAINT organisation_director_designation_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.organisation_director_designation
    ADD CONSTRAINT organisation_director_designation_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.organisation_director_events
    ADD CONSTRAINT organisation_director_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.organisation_director_events
    ADD CONSTRAINT organisation_director_events_designation_id_fkey FOREIGN KEY (designation_id) REFERENCES public.organisation_director_designation(id);

ALTER TABLE ONLY public.project_budget_allocations
    ADD CONSTRAINT project_budget_allocations_bo_id_fkey FOREIGN KEY (bo_id) REFERENCES public.purchase_bo_lists(id);

ALTER TABLE ONLY public.project_budget_allocations
    ADD CONSTRAINT project_budget_allocations_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.project_budget_allocations
    ADD CONSTRAINT project_budget_allocations_head_id_project_id_fkey FOREIGN KEY (head_id, project_id) REFERENCES public.project_budget_heads(id, project_id);

ALTER TABLE ONLY public.project_budget_allocations
    ADD CONSTRAINT project_budget_allocations_line_id_fkey FOREIGN KEY (line_id) REFERENCES public.purchase_bo_lines(id);

ALTER TABLE ONLY public.project_budget_allocations
    ADD CONSTRAINT project_budget_allocations_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.project_budget_allocations
    ADD CONSTRAINT project_budget_allocations_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.project_budgets(project_id);

ALTER TABLE ONLY public.project_budget_allocations
    ADD CONSTRAINT project_budget_allocations_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.project_budget_events
    ADD CONSTRAINT project_budget_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.project_budget_events
    ADD CONSTRAINT project_budget_events_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.project_budgets(project_id);

ALTER TABLE ONLY public.project_budget_heads
    ADD CONSTRAINT project_budget_heads_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.project_budget_heads
    ADD CONSTRAINT project_budget_heads_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.project_budget_heads
    ADD CONSTRAINT project_budget_heads_parent_id_project_id_fkey FOREIGN KEY (parent_id, project_id) REFERENCES public.project_budget_heads(id, project_id) DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE ONLY public.project_budget_heads
    ADD CONSTRAINT project_budget_heads_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.project_budgets(project_id);

ALTER TABLE ONLY public.project_budget_heads
    ADD CONSTRAINT project_budget_heads_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.project_budgets
    ADD CONSTRAINT project_budgets_approval_decision_id_fkey FOREIGN KEY (approval_decision_id) REFERENCES public.finance_budget_decisions(id);

ALTER TABLE ONLY public.project_budgets
    ADD CONSTRAINT project_budgets_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.project_budgets
    ADD CONSTRAINT project_budgets_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.project_budgets
    ADD CONSTRAINT project_budgets_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.project_budgets
    ADD CONSTRAINT project_budgets_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.project_imports
    ADD CONSTRAINT project_imports_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.project_imports
    ADD CONSTRAINT project_imports_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.project_manager_history
    ADD CONSTRAINT project_manager_history_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.project_manager_history
    ADD CONSTRAINT project_manager_history_previous_manager_id_fkey FOREIGN KEY (previous_manager_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.project_manager_history
    ADD CONSTRAINT project_manager_history_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.project_manager_history
    ADD CONSTRAINT project_manager_history_project_manager_id_fkey FOREIGN KEY (project_manager_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.project_material_events
    ADD CONSTRAINT project_material_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.project_material_events
    ADD CONSTRAINT project_material_events_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES public.project_material_requirements(id);

ALTER TABLE ONLY public.project_material_requirements
    ADD CONSTRAINT project_material_requirements_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.project_material_requirements
    ADD CONSTRAINT project_material_requirements_item_id_fkey FOREIGN KEY (item_id) REFERENCES public.catalogue_items(id);

ALTER TABLE ONLY public.project_material_requirements
    ADD CONSTRAINT project_material_requirements_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.project_material_requirements
    ADD CONSTRAINT project_material_requirements_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.project_metadata_changes
    ADD CONSTRAINT project_metadata_changes_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.project_metadata_changes
    ADD CONSTRAINT project_metadata_changes_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.project_responsibility_events
    ADD CONSTRAINT project_responsibility_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.project_responsibility_events
    ADD CONSTRAINT project_responsibility_events_profile_id_project_id_fkey FOREIGN KEY (profile_id, project_id) REFERENCES public.project_responsibility_profiles(id, project_id);

ALTER TABLE ONLY public.project_responsibility_events
    ADD CONSTRAINT project_responsibility_events_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.project_responsibility_profiles
    ADD CONSTRAINT project_responsibility_profiles_bundle_id_fkey FOREIGN KEY (bundle_id) REFERENCES auth.roles(id);

ALTER TABLE ONLY public.project_responsibility_profiles
    ADD CONSTRAINT project_responsibility_profiles_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.project_responsibility_profiles
    ADD CONSTRAINT project_responsibility_profiles_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.project_responsibility_profiles
    ADD CONSTRAINT project_responsibility_profiles_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.project_responsibility_profiles
    ADD CONSTRAINT project_responsibility_profiles_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.project_schedule_changes
    ADD CONSTRAINT project_schedule_changes_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.project_schedule_changes
    ADD CONSTRAINT project_schedule_changes_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.projects
    ADD CONSTRAINT projects_client_id_fkey FOREIGN KEY (client_id) REFERENCES public.clients(id);

ALTER TABLE ONLY public.projects
    ADD CONSTRAINT projects_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.projects
    ADD CONSTRAINT projects_operations_manager_id_fkey FOREIGN KEY (operations_manager_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.projects
    ADD CONSTRAINT projects_project_manager_id_fkey FOREIGN KEY (project_manager_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.projects
    ADD CONSTRAINT projects_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_acceptance_events
    ADD CONSTRAINT purchase_acceptance_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_acceptance_events
    ADD CONSTRAINT purchase_acceptance_events_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES public.purchase_acceptances(id);

ALTER TABLE ONLY public.purchase_acceptances
    ADD CONSTRAINT purchase_acceptances_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_acceptances
    ADD CONSTRAINT purchase_acceptances_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.purchase_orders(id);

ALTER TABLE ONLY public.purchase_acceptances
    ADD CONSTRAINT purchase_acceptances_package_id_fkey FOREIGN KEY (package_id) REFERENCES public.purchase_work_packages(id);

ALTER TABLE ONLY public.purchase_acceptances
    ADD CONSTRAINT purchase_acceptances_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_acceptances
    ADD CONSTRAINT purchase_acceptances_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_advances
    ADD CONSTRAINT purchase_advance_decision_fk FOREIGN KEY (decision_id) REFERENCES public.finance_advance_decisions(id);

ALTER TABLE ONLY public.purchase_advance_events
    ADD CONSTRAINT purchase_advance_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_advance_events
    ADD CONSTRAINT purchase_advance_events_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES public.purchase_advances(id);

ALTER TABLE ONLY public.purchase_advances
    ADD CONSTRAINT purchase_advances_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_advances
    ADD CONSTRAINT purchase_advances_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.purchase_orders(id);

ALTER TABLE ONLY public.purchase_advances
    ADD CONSTRAINT purchase_advances_package_id_fkey FOREIGN KEY (package_id) REFERENCES public.purchase_work_packages(id);

ALTER TABLE ONLY public.purchase_advances
    ADD CONSTRAINT purchase_advances_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_advances
    ADD CONSTRAINT purchase_advances_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_award_handoffs
    ADD CONSTRAINT purchase_award_handoffs_bo_id_fkey FOREIGN KEY (bo_id) REFERENCES public.purchase_bo_lists(id);

ALTER TABLE ONLY public.purchase_award_handoffs
    ADD CONSTRAINT purchase_award_handoffs_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_award_handoffs
    ADD CONSTRAINT purchase_award_handoffs_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_award_handoffs
    ADD CONSTRAINT purchase_award_handoffs_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_bo_events
    ADD CONSTRAINT purchase_bo_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_bo_events
    ADD CONSTRAINT purchase_bo_events_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES public.purchase_bo_lists(id);

ALTER TABLE ONLY public.purchase_bo_lines
    ADD CONSTRAINT purchase_bo_lines_bo_id_fkey FOREIGN KEY (bo_id) REFERENCES public.purchase_bo_lists(id);

ALTER TABLE ONLY public.purchase_bo_lines
    ADD CONSTRAINT purchase_bo_lines_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_bo_lines
    ADD CONSTRAINT purchase_bo_lines_demand_id_fkey FOREIGN KEY (demand_id) REFERENCES public.project_material_requirements(id);

ALTER TABLE ONLY public.purchase_bo_lines
    ADD CONSTRAINT purchase_bo_lines_item_id_fkey FOREIGN KEY (item_id) REFERENCES public.catalogue_items(id);

ALTER TABLE ONLY public.purchase_bo_lines
    ADD CONSTRAINT purchase_bo_lines_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_bo_lists
    ADD CONSTRAINT purchase_bo_lists_completed_by_id_fkey FOREIGN KEY (completed_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_bo_lists
    ADD CONSTRAINT purchase_bo_lists_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_bo_lists
    ADD CONSTRAINT purchase_bo_lists_merged_into_id_fkey FOREIGN KEY (merged_into_id) REFERENCES public.purchase_bo_lists(id);

ALTER TABLE ONLY public.purchase_bo_lists
    ADD CONSTRAINT purchase_bo_lists_preparer_id_fkey FOREIGN KEY (preparer_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_bo_lists
    ADD CONSTRAINT purchase_bo_lists_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_bo_lists
    ADD CONSTRAINT purchase_bo_lists_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_client_clearance_events
    ADD CONSTRAINT purchase_client_clearance_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_client_clearance_events
    ADD CONSTRAINT purchase_client_clearance_events_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES public.purchase_client_clearances(id);

ALTER TABLE ONLY public.purchase_client_clearances
    ADD CONSTRAINT purchase_client_clearances_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_client_clearances
    ADD CONSTRAINT purchase_client_clearances_package_id_fkey FOREIGN KEY (package_id) REFERENCES public.purchase_work_packages(id);

ALTER TABLE ONLY public.purchase_client_clearances
    ADD CONSTRAINT purchase_client_clearances_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_client_clearances
    ADD CONSTRAINT purchase_client_clearances_quality_id_fkey FOREIGN KEY (quality_id) REFERENCES public.purchase_quality_cases(id);

ALTER TABLE ONLY public.purchase_client_clearances
    ADD CONSTRAINT purchase_client_clearances_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_dispatch_events
    ADD CONSTRAINT purchase_dispatch_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_dispatch_events
    ADD CONSTRAINT purchase_dispatch_events_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES public.purchase_dispatches(id);

ALTER TABLE ONLY public.purchase_dispatches
    ADD CONSTRAINT purchase_dispatches_clearance_id_fkey FOREIGN KEY (clearance_id) REFERENCES public.purchase_client_clearances(id);

ALTER TABLE ONLY public.purchase_dispatches
    ADD CONSTRAINT purchase_dispatches_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_dispatches
    ADD CONSTRAINT purchase_dispatches_package_id_fkey FOREIGN KEY (package_id) REFERENCES public.purchase_work_packages(id);

ALTER TABLE ONLY public.purchase_dispatches
    ADD CONSTRAINT purchase_dispatches_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_dispatches
    ADD CONSTRAINT purchase_dispatches_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_engineer_confirmations
    ADD CONSTRAINT purchase_engineer_confirmations_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_engineer_confirmations_events
    ADD CONSTRAINT purchase_engineer_confirmations_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_engineer_confirmations_events
    ADD CONSTRAINT purchase_engineer_confirmations_events_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES public.purchase_engineer_confirmations(id);

ALTER TABLE ONLY public.purchase_engineer_confirmations
    ADD CONSTRAINT purchase_engineer_confirmations_grn_id_fkey FOREIGN KEY (grn_id) REFERENCES public.purchase_grns(id);

ALTER TABLE ONLY public.purchase_engineer_confirmations
    ADD CONSTRAINT purchase_engineer_confirmations_package_id_fkey FOREIGN KEY (package_id) REFERENCES public.purchase_work_packages(id);

ALTER TABLE ONLY public.purchase_engineer_confirmations
    ADD CONSTRAINT purchase_engineer_confirmations_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_engineer_confirmations
    ADD CONSTRAINT purchase_engineer_confirmations_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_fms_owner_history
    ADD CONSTRAINT purchase_fms_owner_history_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_fms_owner_history
    ADD CONSTRAINT purchase_fms_owner_history_previous_manager_id_fkey FOREIGN KEY (previous_manager_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_fms_owner_history
    ADD CONSTRAINT purchase_fms_owner_history_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.purchase_fms_workspaces(project_id);

ALTER TABLE ONLY public.purchase_fms_owner_history
    ADD CONSTRAINT purchase_fms_owner_history_purchase_manager_id_fkey FOREIGN KEY (purchase_manager_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_fms_workspaces
    ADD CONSTRAINT purchase_fms_workspaces_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_fms_workspaces
    ADD CONSTRAINT purchase_fms_workspaces_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_fms_workspaces
    ADD CONSTRAINT purchase_fms_workspaces_purchase_manager_id_fkey FOREIGN KEY (purchase_manager_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_fms_workspaces
    ADD CONSTRAINT purchase_fms_workspaces_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_grns
    ADD CONSTRAINT purchase_grns_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_grns_events
    ADD CONSTRAINT purchase_grns_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_grns_events
    ADD CONSTRAINT purchase_grns_events_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES public.purchase_grns(id);

ALTER TABLE ONLY public.purchase_grns
    ADD CONSTRAINT purchase_grns_package_id_fkey FOREIGN KEY (package_id) REFERENCES public.purchase_work_packages(id);

ALTER TABLE ONLY public.purchase_grns
    ADD CONSTRAINT purchase_grns_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_grns
    ADD CONSTRAINT purchase_grns_receipt_id_fkey FOREIGN KEY (receipt_id) REFERENCES public.purchase_site_receipts(id);

ALTER TABLE ONLY public.purchase_grns
    ADD CONSTRAINT purchase_grns_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_group_quotations
    ADD CONSTRAINT purchase_group_quotations_bo_id_fkey FOREIGN KEY (bo_id) REFERENCES public.purchase_bo_lists(id);

ALTER TABLE ONLY public.purchase_group_quotations
    ADD CONSTRAINT purchase_group_quotations_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_group_quotations
    ADD CONSTRAINT purchase_group_quotations_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_group_quotations
    ADD CONSTRAINT purchase_group_quotations_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_hold_events
    ADD CONSTRAINT purchase_hold_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_hold_events
    ADD CONSTRAINT purchase_hold_events_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES public.purchase_holds(id);

ALTER TABLE ONLY public.purchase_holds
    ADD CONSTRAINT purchase_holds_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_holds
    ADD CONSTRAINT purchase_holds_doer_id_fkey FOREIGN KEY (doer_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_holds
    ADD CONSTRAINT purchase_holds_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_holds
    ADD CONSTRAINT purchase_holds_resolver_id_fkey FOREIGN KEY (resolver_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_holds
    ADD CONSTRAINT purchase_holds_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_indent_dispatch_notices
    ADD CONSTRAINT purchase_indent_dispatch_notices_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_indent_dispatch_notices
    ADD CONSTRAINT purchase_indent_dispatch_notices_document_id_fkey FOREIGN KEY (document_id) REFERENCES public.purchase_indent_documents(id);

ALTER TABLE ONLY public.purchase_indent_dispatch_notices
    ADD CONSTRAINT purchase_indent_dispatch_notices_recipient_id_fkey FOREIGN KEY (recipient_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_indent_dispatch_notices
    ADD CONSTRAINT purchase_indent_dispatch_notices_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_indent_document_events
    ADD CONSTRAINT purchase_indent_document_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_indent_document_events
    ADD CONSTRAINT purchase_indent_document_events_document_id_fkey FOREIGN KEY (document_id) REFERENCES public.purchase_indent_documents(id);

ALTER TABLE ONLY public.purchase_indent_documents
    ADD CONSTRAINT purchase_indent_documents_bo_id_fkey FOREIGN KEY (bo_id) REFERENCES public.purchase_bo_lists(id);

ALTER TABLE ONLY public.purchase_indent_documents
    ADD CONSTRAINT purchase_indent_documents_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_indent_documents
    ADD CONSTRAINT purchase_indent_documents_group_id_fkey FOREIGN KEY (group_id) REFERENCES public.purchase_indent_group_entries(id);

ALTER TABLE ONLY public.purchase_indent_documents
    ADD CONSTRAINT purchase_indent_documents_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_indent_documents
    ADD CONSTRAINT purchase_indent_documents_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_indent_documents
    ADD CONSTRAINT purchase_indent_documents_vendor_id_fkey FOREIGN KEY (vendor_id) REFERENCES public.suppliers(id);

ALTER TABLE ONLY public.purchase_indent_events
    ADD CONSTRAINT purchase_indent_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_indent_events
    ADD CONSTRAINT purchase_indent_events_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES public.purchase_indents(id);

ALTER TABLE ONLY public.purchase_indent_group_entries
    ADD CONSTRAINT purchase_indent_group_entries_bo_id_fkey FOREIGN KEY (bo_id) REFERENCES public.purchase_indent_group_sets(bo_id);

ALTER TABLE ONLY public.purchase_indent_group_entries
    ADD CONSTRAINT purchase_indent_group_entries_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_indent_group_entries
    ADD CONSTRAINT purchase_indent_group_entries_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_indent_group_events
    ADD CONSTRAINT purchase_indent_group_events_bo_id_fkey FOREIGN KEY (bo_id) REFERENCES public.purchase_indent_group_sets(bo_id);

ALTER TABLE ONLY public.purchase_indent_group_events
    ADD CONSTRAINT purchase_indent_group_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_indent_group_sets
    ADD CONSTRAINT purchase_indent_group_sets_bo_id_fkey FOREIGN KEY (bo_id) REFERENCES public.purchase_bo_lists(id);

ALTER TABLE ONLY public.purchase_indent_group_sets
    ADD CONSTRAINT purchase_indent_group_sets_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_indent_group_sets
    ADD CONSTRAINT purchase_indent_group_sets_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_indent_group_sets
    ADD CONSTRAINT purchase_indent_group_sets_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_indent_legacy_terms
    ADD CONSTRAINT purchase_indent_legacy_terms_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_indent_legacy_terms
    ADD CONSTRAINT purchase_indent_legacy_terms_document_id_fkey FOREIGN KEY (document_id) REFERENCES public.purchase_indent_documents(id);

ALTER TABLE ONLY public.purchase_indent_releases
    ADD CONSTRAINT purchase_indent_releases_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_indent_releases
    ADD CONSTRAINT purchase_indent_releases_demand_id_fkey FOREIGN KEY (demand_id) REFERENCES public.project_material_requirements(id);

ALTER TABLE ONLY public.purchase_indent_releases
    ADD CONSTRAINT purchase_indent_releases_indent_id_fkey FOREIGN KEY (indent_id) REFERENCES public.purchase_indents(id);

ALTER TABLE ONLY public.purchase_indent_templates
    ADD CONSTRAINT purchase_indent_templates_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_indents
    ADD CONSTRAINT purchase_indents_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_indents
    ADD CONSTRAINT purchase_indents_doer_id_fkey FOREIGN KEY (doer_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_indents
    ADD CONSTRAINT purchase_indents_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_indents
    ADD CONSTRAINT purchase_indents_resolver_id_fkey FOREIGN KEY (resolver_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_indents
    ADD CONSTRAINT purchase_indents_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_inspection_events
    ADD CONSTRAINT purchase_inspection_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_inspection_events
    ADD CONSTRAINT purchase_inspection_events_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES public.purchase_inspections(id);

ALTER TABLE ONLY public.purchase_inspection_report_events
    ADD CONSTRAINT purchase_inspection_report_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_inspection_report_events
    ADD CONSTRAINT purchase_inspection_report_events_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES public.purchase_inspection_reports(id);

ALTER TABLE ONLY public.purchase_inspection_reports
    ADD CONSTRAINT purchase_inspection_reports_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_inspection_reports
    ADD CONSTRAINT purchase_inspection_reports_inspection_id_fkey FOREIGN KEY (inspection_id) REFERENCES public.purchase_inspections(id);

ALTER TABLE ONLY public.purchase_inspection_reports
    ADD CONSTRAINT purchase_inspection_reports_package_id_fkey FOREIGN KEY (package_id) REFERENCES public.purchase_work_packages(id);

ALTER TABLE ONLY public.purchase_inspection_reports
    ADD CONSTRAINT purchase_inspection_reports_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_inspection_reports
    ADD CONSTRAINT purchase_inspection_reports_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_inspections
    ADD CONSTRAINT purchase_inspections_acceptance_id_fkey FOREIGN KEY (acceptance_id) REFERENCES public.purchase_acceptances(id);

ALTER TABLE ONLY public.purchase_inspections
    ADD CONSTRAINT purchase_inspections_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_inspections
    ADD CONSTRAINT purchase_inspections_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.purchase_orders(id);

ALTER TABLE ONLY public.purchase_inspections
    ADD CONSTRAINT purchase_inspections_package_id_fkey FOREIGN KEY (package_id) REFERENCES public.purchase_work_packages(id);

ALTER TABLE ONLY public.purchase_inspections
    ADD CONSTRAINT purchase_inspections_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_inspections
    ADD CONSTRAINT purchase_inspections_technical_id_fkey FOREIGN KEY (technical_id) REFERENCES public.purchase_technical_cases(id);

ALTER TABLE ONLY public.purchase_inspections
    ADD CONSTRAINT purchase_inspections_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_loi_document_events
    ADD CONSTRAINT purchase_loi_document_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_loi_document_events
    ADD CONSTRAINT purchase_loi_document_events_document_id_fkey FOREIGN KEY (document_id) REFERENCES public.purchase_loi_documents(id);

ALTER TABLE ONLY public.purchase_loi_documents
    ADD CONSTRAINT purchase_loi_documents_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_loi_documents
    ADD CONSTRAINT purchase_loi_documents_handoff_id_fkey FOREIGN KEY (handoff_id) REFERENCES public.purchase_award_handoffs(id);

ALTER TABLE ONLY public.purchase_loi_documents
    ADD CONSTRAINT purchase_loi_documents_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_loi_documents
    ADD CONSTRAINT purchase_loi_documents_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_loi_reviews
    ADD CONSTRAINT purchase_loi_reviews_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_loi_reviews
    ADD CONSTRAINT purchase_loi_reviews_document_id_fkey FOREIGN KEY (document_id) REFERENCES public.purchase_loi_documents(id);

ALTER TABLE ONLY public.purchase_loi_reviews
    ADD CONSTRAINT purchase_loi_reviews_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_order_client_file_events
    ADD CONSTRAINT purchase_order_client_file_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_order_client_file_events
    ADD CONSTRAINT purchase_order_client_file_events_file_id_fkey FOREIGN KEY (file_id) REFERENCES public.purchase_order_client_files(id);

ALTER TABLE ONLY public.purchase_order_client_file_events
    ADD CONSTRAINT purchase_order_client_file_events_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.purchase_orders(id);

ALTER TABLE ONLY public.purchase_order_client_files
    ADD CONSTRAINT purchase_order_client_files_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_order_client_files
    ADD CONSTRAINT purchase_order_client_files_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.purchase_orders(id);

ALTER TABLE ONLY public.purchase_order_client_files
    ADD CONSTRAINT purchase_order_client_files_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_order_document_events
    ADD CONSTRAINT purchase_order_document_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_order_document_events
    ADD CONSTRAINT purchase_order_document_events_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.purchase_orders(id);

ALTER TABLE ONLY public.purchase_order_events
    ADD CONSTRAINT purchase_order_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_order_events
    ADD CONSTRAINT purchase_order_events_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES public.purchase_orders(id);

ALTER TABLE ONLY public.purchase_orders
    ADD CONSTRAINT purchase_orders_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_orders
    ADD CONSTRAINT purchase_orders_package_id_fkey FOREIGN KEY (package_id) REFERENCES public.purchase_work_packages(id);

ALTER TABLE ONLY public.purchase_orders
    ADD CONSTRAINT purchase_orders_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_orders
    ADD CONSTRAINT purchase_orders_sourcing_id_fkey FOREIGN KEY (sourcing_id) REFERENCES public.purchase_sourcing_cases(id);

ALTER TABLE ONLY public.purchase_orders
    ADD CONSTRAINT purchase_orders_technical_id_fkey FOREIGN KEY (technical_id) REFERENCES public.purchase_technical_cases(id);

ALTER TABLE ONLY public.purchase_orders
    ADD CONSTRAINT purchase_orders_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_project_vendors
    ADD CONSTRAINT purchase_project_vendors_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_project_vendors
    ADD CONSTRAINT purchase_project_vendors_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_quality_cases
    ADD CONSTRAINT purchase_quality_cases_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_quality_cases
    ADD CONSTRAINT purchase_quality_cases_package_id_fkey FOREIGN KEY (package_id) REFERENCES public.purchase_work_packages(id);

ALTER TABLE ONLY public.purchase_quality_cases
    ADD CONSTRAINT purchase_quality_cases_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_quality_cases
    ADD CONSTRAINT purchase_quality_cases_report_id_fkey FOREIGN KEY (report_id) REFERENCES public.purchase_inspection_reports(id);

ALTER TABLE ONLY public.purchase_quality_cases
    ADD CONSTRAINT purchase_quality_cases_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_quality_certificates
    ADD CONSTRAINT purchase_quality_certificates_case_id_fkey FOREIGN KEY (case_id) REFERENCES public.purchase_quality_cases(id);

ALTER TABLE ONLY public.purchase_quality_certificates
    ADD CONSTRAINT purchase_quality_certificates_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_quality_certificates
    ADD CONSTRAINT purchase_quality_certificates_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_quality_certificates
    ADD CONSTRAINT purchase_quality_certificates_verified_by_id_fkey FOREIGN KEY (verified_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_quality_events
    ADD CONSTRAINT purchase_quality_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_quality_events
    ADD CONSTRAINT purchase_quality_events_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES public.purchase_quality_cases(id);

ALTER TABLE ONLY public.purchase_quotation_attachments
    ADD CONSTRAINT purchase_quotation_attachments_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_quotation_attachments
    ADD CONSTRAINT purchase_quotation_attachments_document_id_fkey FOREIGN KEY (document_id) REFERENCES public.purchase_indent_documents(id);

ALTER TABLE ONLY public.purchase_quotation_attachments
    ADD CONSTRAINT purchase_quotation_attachments_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_quotation_events
    ADD CONSTRAINT purchase_quotation_events_bo_id_fkey FOREIGN KEY (bo_id) REFERENCES public.purchase_group_quotations(bo_id);

ALTER TABLE ONLY public.purchase_quotation_events
    ADD CONSTRAINT purchase_quotation_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_quotation_evidence_events
    ADD CONSTRAINT purchase_quotation_evidence_events_attachment_id_fkey FOREIGN KEY (attachment_id) REFERENCES public.purchase_quotation_attachments(id);

ALTER TABLE ONLY public.purchase_quotation_evidence_events
    ADD CONSTRAINT purchase_quotation_evidence_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_quotation_revisions
    ADD CONSTRAINT purchase_quotation_revisions_bo_id_fkey FOREIGN KEY (bo_id) REFERENCES public.purchase_group_quotations(bo_id);

ALTER TABLE ONLY public.purchase_quotation_revisions
    ADD CONSTRAINT purchase_quotation_revisions_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_quotation_revisions
    ADD CONSTRAINT purchase_quotation_revisions_document_id_attachment_id_fkey FOREIGN KEY (document_id, attachment_id) REFERENCES public.purchase_quotation_attachments(document_id, id);

ALTER TABLE ONLY public.purchase_quotation_revisions
    ADD CONSTRAINT purchase_quotation_revisions_document_id_fkey FOREIGN KEY (document_id) REFERENCES public.purchase_indent_documents(id);

ALTER TABLE ONLY public.purchase_quotation_shortlist_drafts
    ADD CONSTRAINT purchase_quotation_shortlist_drafts_bo_id_fkey FOREIGN KEY (bo_id) REFERENCES public.purchase_bo_lists(id);

ALTER TABLE ONLY public.purchase_quotation_shortlist_drafts
    ADD CONSTRAINT purchase_quotation_shortlist_drafts_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_quotation_shortlist_drafts
    ADD CONSTRAINT purchase_quotation_shortlist_drafts_group_id_fkey FOREIGN KEY (group_id) REFERENCES public.purchase_indent_group_entries(id);

ALTER TABLE ONLY public.purchase_quotation_shortlist_drafts
    ADD CONSTRAINT purchase_quotation_shortlist_drafts_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_quotation_shortlist_drafts
    ADD CONSTRAINT purchase_quotation_shortlist_drafts_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_quotation_shortlist_history
    ADD CONSTRAINT purchase_quotation_shortlist_history_bo_id_fkey FOREIGN KEY (bo_id) REFERENCES public.purchase_bo_lists(id);

ALTER TABLE ONLY public.purchase_quotation_shortlist_history
    ADD CONSTRAINT purchase_quotation_shortlist_history_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_quotation_shortlist_history
    ADD CONSTRAINT purchase_quotation_shortlist_history_group_id_fkey FOREIGN KEY (group_id) REFERENCES public.purchase_indent_group_entries(id);

ALTER TABLE ONLY public.purchase_quotation_shortlist_history
    ADD CONSTRAINT purchase_quotation_shortlist_history_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_quotation_work_tasks
    ADD CONSTRAINT purchase_quotation_work_tasks_bo_id_fkey FOREIGN KEY (bo_id) REFERENCES public.purchase_bo_lists(id);

ALTER TABLE ONLY public.purchase_quotation_work_tasks
    ADD CONSTRAINT purchase_quotation_work_tasks_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_quotation_work_tasks
    ADD CONSTRAINT purchase_quotation_work_tasks_group_id_fkey FOREIGN KEY (group_id) REFERENCES public.purchase_indent_group_entries(id);

ALTER TABLE ONLY public.purchase_quotation_work_tasks
    ADD CONSTRAINT purchase_quotation_work_tasks_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_quotation_work_tasks
    ADD CONSTRAINT purchase_quotation_work_tasks_recipient_id_fkey FOREIGN KEY (recipient_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_quotation_work_tasks
    ADD CONSTRAINT purchase_quotation_work_tasks_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_site_receipts
    ADD CONSTRAINT purchase_site_receipts_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_site_receipts
    ADD CONSTRAINT purchase_site_receipts_dispatch_id_fkey FOREIGN KEY (dispatch_id) REFERENCES public.purchase_dispatches(id);

ALTER TABLE ONLY public.purchase_site_receipts_events
    ADD CONSTRAINT purchase_site_receipts_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_site_receipts_events
    ADD CONSTRAINT purchase_site_receipts_events_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES public.purchase_site_receipts(id);

ALTER TABLE ONLY public.purchase_site_receipts
    ADD CONSTRAINT purchase_site_receipts_package_id_fkey FOREIGN KEY (package_id) REFERENCES public.purchase_work_packages(id);

ALTER TABLE ONLY public.purchase_site_receipts
    ADD CONSTRAINT purchase_site_receipts_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_site_receipts
    ADD CONSTRAINT purchase_site_receipts_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_sourcing_allocations
    ADD CONSTRAINT purchase_sourcing_allocations_case_id_fkey FOREIGN KEY (case_id) REFERENCES public.purchase_sourcing_cases(id);

ALTER TABLE ONLY public.purchase_sourcing_allocations
    ADD CONSTRAINT purchase_sourcing_allocations_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_sourcing_allocations
    ADD CONSTRAINT purchase_sourcing_allocations_demand_id_fkey FOREIGN KEY (demand_id) REFERENCES public.project_material_requirements(id);

ALTER TABLE ONLY public.purchase_sourcing_allocations
    ADD CONSTRAINT purchase_sourcing_allocations_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_sourcing_allocations
    ADD CONSTRAINT purchase_sourcing_allocations_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_sourcing_cases
    ADD CONSTRAINT purchase_sourcing_cases_approver_id_fkey FOREIGN KEY (approver_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_sourcing_cases
    ADD CONSTRAINT purchase_sourcing_cases_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_sourcing_cases
    ADD CONSTRAINT purchase_sourcing_cases_package_id_fkey FOREIGN KEY (package_id) REFERENCES public.purchase_work_packages(id);

ALTER TABLE ONLY public.purchase_sourcing_cases
    ADD CONSTRAINT purchase_sourcing_cases_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_sourcing_cases
    ADD CONSTRAINT purchase_sourcing_cases_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_sourcing_events
    ADD CONSTRAINT purchase_sourcing_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_sourcing_events
    ADD CONSTRAINT purchase_sourcing_events_id_fkey FOREIGN KEY (id) REFERENCES public.business_events(id) DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE ONLY public.purchase_sourcing_events
    ADD CONSTRAINT purchase_sourcing_events_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES public.purchase_sourcing_cases(id);

ALTER TABLE ONLY public.purchase_technical_cases
    ADD CONSTRAINT purchase_technical_cases_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_technical_cases
    ADD CONSTRAINT purchase_technical_cases_package_id_fkey FOREIGN KEY (package_id) REFERENCES public.purchase_work_packages(id);

ALTER TABLE ONLY public.purchase_technical_cases
    ADD CONSTRAINT purchase_technical_cases_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_technical_cases
    ADD CONSTRAINT purchase_technical_cases_source_case_id_fkey FOREIGN KEY (source_case_id) REFERENCES public.purchase_sourcing_cases(id);

ALTER TABLE ONLY public.purchase_technical_cases
    ADD CONSTRAINT purchase_technical_cases_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_technical_events
    ADD CONSTRAINT purchase_technical_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_technical_events
    ADD CONSTRAINT purchase_technical_events_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES public.purchase_technical_cases(id);

ALTER TABLE ONLY public.purchase_technical_file_events
    ADD CONSTRAINT purchase_technical_file_events_case_id_fkey FOREIGN KEY (case_id) REFERENCES public.purchase_technical_cases(id);

ALTER TABLE ONLY public.purchase_technical_file_events
    ADD CONSTRAINT purchase_technical_file_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_technical_file_events
    ADD CONSTRAINT purchase_technical_file_events_file_id_case_id_fkey FOREIGN KEY (file_id, case_id) REFERENCES public.purchase_technical_files(id, case_id);

ALTER TABLE ONLY public.purchase_technical_files
    ADD CONSTRAINT purchase_technical_files_case_id_fkey FOREIGN KEY (case_id) REFERENCES public.purchase_technical_cases(id);

ALTER TABLE ONLY public.purchase_technical_files
    ADD CONSTRAINT purchase_technical_files_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_technical_files
    ADD CONSTRAINT purchase_technical_files_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_type_indent_drafts
    ADD CONSTRAINT purchase_type_indent_drafts_category_code_template_id_temp_fkey FOREIGN KEY (category_code, template_id, template_version) REFERENCES public.purchase_indent_templates(category_code, template_id, version);

ALTER TABLE ONLY public.purchase_type_indent_drafts
    ADD CONSTRAINT purchase_type_indent_drafts_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_type_indent_drafts
    ADD CONSTRAINT purchase_type_indent_drafts_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_work_package_events
    ADD CONSTRAINT purchase_work_package_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_work_package_events
    ADD CONSTRAINT purchase_work_package_events_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES public.purchase_work_packages(id);

ALTER TABLE ONLY public.purchase_work_packages
    ADD CONSTRAINT purchase_work_packages_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.purchase_work_packages
    ADD CONSTRAINT purchase_work_packages_demand_id_fkey FOREIGN KEY (demand_id) REFERENCES public.project_material_requirements(id);

ALTER TABLE ONLY public.purchase_work_packages
    ADD CONSTRAINT purchase_work_packages_indent_id_fkey FOREIGN KEY (indent_id) REFERENCES public.purchase_indents(id);

ALTER TABLE ONLY public.purchase_work_packages
    ADD CONSTRAINT purchase_work_packages_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.purchase_work_packages
    ADD CONSTRAINT purchase_work_packages_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.quotation_approval_events
    ADD CONSTRAINT quotation_approval_events_bo_id_fkey FOREIGN KEY (bo_id) REFERENCES public.quotation_approval_sets(bo_id);

ALTER TABLE ONLY public.quotation_approval_events
    ADD CONSTRAINT quotation_approval_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.quotation_approval_sets
    ADD CONSTRAINT quotation_approval_sets_bo_id_fkey FOREIGN KEY (bo_id) REFERENCES public.purchase_bo_lists(id);

ALTER TABLE ONLY public.quotation_approval_sets
    ADD CONSTRAINT quotation_approval_sets_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.quotation_approval_sets
    ADD CONSTRAINT quotation_approval_sets_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY public.quotation_approval_sets
    ADD CONSTRAINT quotation_approval_sets_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.stored_document_events
    ADD CONSTRAINT stored_document_events_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.stored_document_events
    ADD CONSTRAINT stored_document_events_document_id_fkey FOREIGN KEY (document_id) REFERENCES public.stored_documents(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.stored_documents
    ADD CONSTRAINT stored_documents_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.stored_documents
    ADD CONSTRAINT stored_documents_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.category_subcategories
    ADD CONSTRAINT subcategory_inherits_unit FOREIGN KEY (category_id, unit_id) REFERENCES public.categories(id, unit_id);

ALTER TABLE ONLY public.supplier_changes
    ADD CONSTRAINT supplier_changes_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.supplier_changes
    ADD CONSTRAINT supplier_changes_supplier_id_fkey FOREIGN KEY (supplier_id) REFERENCES public.suppliers(id);

ALTER TABLE ONLY public.supplier_document_approval_changes
    ADD CONSTRAINT supplier_document_approval_changes_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.supplier_document_approval_changes
    ADD CONSTRAINT supplier_document_approval_changes_document_id_fkey FOREIGN KEY (document_id) REFERENCES public.supplier_documents(id);

ALTER TABLE ONLY public.supplier_document_approvals
    ADD CONSTRAINT supplier_document_approvals_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.supplier_document_approvals
    ADD CONSTRAINT supplier_document_approvals_document_id_fkey FOREIGN KEY (document_id) REFERENCES public.supplier_documents(id);

ALTER TABLE ONLY public.supplier_document_approvals
    ADD CONSTRAINT supplier_document_approvals_supplier_id_fkey FOREIGN KEY (supplier_id) REFERENCES public.suppliers(id);

ALTER TABLE ONLY public.supplier_document_approvals
    ADD CONSTRAINT supplier_document_approvals_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.supplier_document_changes
    ADD CONSTRAINT supplier_document_changes_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.supplier_document_changes
    ADD CONSTRAINT supplier_document_changes_document_id_fkey FOREIGN KEY (document_id) REFERENCES public.supplier_documents(id);

ALTER TABLE ONLY public.supplier_document_tasks
    ADD CONSTRAINT supplier_document_tasks_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.supplier_document_tasks
    ADD CONSTRAINT supplier_document_tasks_document_id_fkey FOREIGN KEY (document_id) REFERENCES public.supplier_documents(id);

ALTER TABLE ONLY public.supplier_document_tasks
    ADD CONSTRAINT supplier_document_tasks_supplier_id_fkey FOREIGN KEY (supplier_id) REFERENCES public.suppliers(id);

ALTER TABLE ONLY public.supplier_document_tasks
    ADD CONSTRAINT supplier_document_tasks_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.supplier_document_tasks
    ADD CONSTRAINT supplier_document_tasks_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.supplier_documents
    ADD CONSTRAINT supplier_documents_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.supplier_documents
    ADD CONSTRAINT supplier_documents_supplier_id_fkey FOREIGN KEY (supplier_id) REFERENCES public.suppliers(id);

ALTER TABLE ONLY public.supplier_documents
    ADD CONSTRAINT supplier_documents_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.supplier_review_assignment_changes
    ADD CONSTRAINT supplier_review_assignment_changes_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.supplier_review_assignment_changes
    ADD CONSTRAINT supplier_review_assignment_changes_supplier_id_fkey FOREIGN KEY (supplier_id) REFERENCES public.suppliers(id);

ALTER TABLE ONLY public.supplier_review_assignments
    ADD CONSTRAINT supplier_review_assignments_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.supplier_review_assignments
    ADD CONSTRAINT supplier_review_assignments_supplier_id_fkey FOREIGN KEY (supplier_id) REFERENCES public.suppliers(id);

ALTER TABLE ONLY public.supplier_review_assignments
    ADD CONSTRAINT supplier_review_assignments_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.supplier_review_assignments
    ADD CONSTRAINT supplier_review_assignments_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.supplier_verification_history
    ADD CONSTRAINT supplier_verification_history_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.supplier_verification_history
    ADD CONSTRAINT supplier_verification_history_supplier_id_fkey FOREIGN KEY (supplier_id) REFERENCES public.suppliers(id);

ALTER TABLE ONLY public.supplier_verifications
    ADD CONSTRAINT supplier_verifications_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.supplier_verifications
    ADD CONSTRAINT supplier_verifications_supplier_id_fkey FOREIGN KEY (supplier_id) REFERENCES public.suppliers(id);

ALTER TABLE ONLY public.supplier_verifications
    ADD CONSTRAINT supplier_verifications_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.suppliers
    ADD CONSTRAINT suppliers_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.suppliers
    ADD CONSTRAINT suppliers_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.template_fields
    ADD CONSTRAINT template_fields_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.template_fields
    ADD CONSTRAINT template_fields_template_id_fkey FOREIGN KEY (template_id) REFERENCES public.configuration_templates(id);

ALTER TABLE ONLY public.template_fields
    ADD CONSTRAINT template_fields_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.template_options
    ADD CONSTRAINT template_options_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.template_options
    ADD CONSTRAINT template_options_field_id_fkey FOREIGN KEY (field_id) REFERENCES public.template_fields(id);

ALTER TABLE ONLY public.template_options
    ADD CONSTRAINT template_options_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.template_publications
    ADD CONSTRAINT template_publications_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.template_publications
    ADD CONSTRAINT template_publications_previous_template_id_fkey FOREIGN KEY (previous_template_id) REFERENCES public.configuration_templates(id);

ALTER TABLE ONLY public.template_publications
    ADD CONSTRAINT template_publications_template_id_fkey FOREIGN KEY (template_id) REFERENCES public.configuration_templates(id);

ALTER TABLE ONLY public.template_publications
    ADD CONSTRAINT template_publications_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.work_calendar_changes
    ADD CONSTRAINT work_calendar_changes_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.work_calendar_changes
    ADD CONSTRAINT work_calendar_changes_holiday_id_fkey FOREIGN KEY (holiday_id) REFERENCES public.work_calendar_holidays(id);

ALTER TABLE ONLY public.work_calendar_holidays
    ADD CONSTRAINT work_calendar_holidays_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY public.work_calendar_holidays
    ADD CONSTRAINT work_calendar_holidays_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.bid_costings
    ADD CONSTRAINT bid_costings_approved_by_id_fkey FOREIGN KEY (approved_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.bid_costings
    ADD CONSTRAINT bid_costings_draft_id_fkey FOREIGN KEY (draft_id) REFERENCES tender.document_drafts(id);

ALTER TABLE ONLY tender.bid_costings
    ADD CONSTRAINT bid_costings_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY tender.bid_items
    ADD CONSTRAINT bid_items_draft_id_fkey FOREIGN KEY (draft_id) REFERENCES tender.document_drafts(id);

ALTER TABLE ONLY tender.bidding_authorizations
    ADD CONSTRAINT bidding_authorizations_authorized_by_id_fkey FOREIGN KEY (authorized_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.bidding_authorizations
    ADD CONSTRAINT bidding_authorizations_draft_id_fkey FOREIGN KEY (draft_id) REFERENCES tender.document_drafts(id);

ALTER TABLE ONLY tender.bidding_authorizations
    ADD CONSTRAINT bidding_authorizations_manager_id_fkey FOREIGN KEY (manager_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.bidding_authorizations
    ADD CONSTRAINT bidding_authorizations_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY tender.document_drafts
    ADD CONSTRAINT document_drafts_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.document_drafts
    ADD CONSTRAINT document_drafts_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.document_drafts
    ADD CONSTRAINT document_drafts_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.draft_document_removals
    ADD CONSTRAINT draft_document_removals_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.draft_document_removals
    ADD CONSTRAINT draft_document_removals_draft_id_fkey FOREIGN KEY (draft_id) REFERENCES tender.document_drafts(id);

ALTER TABLE ONLY tender.draft_document_removals
    ADD CONSTRAINT draft_document_removals_member_id_fkey FOREIGN KEY (member_id) REFERENCES public.document_package_members(id);

ALTER TABLE ONLY tender.draft_events
    ADD CONSTRAINT draft_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.draft_events
    ADD CONSTRAINT draft_events_draft_id_fkey FOREIGN KEY (draft_id) REFERENCES tender.document_drafts(id);

ALTER TABLE ONLY tender.draft_events
    ADD CONSTRAINT draft_events_receipt_id_fkey FOREIGN KEY (receipt_id) REFERENCES public.document_package_receipts(id);

ALTER TABLE ONLY tender.draft_sources
    ADD CONSTRAINT draft_sources_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.draft_sources
    ADD CONSTRAINT draft_sources_document_id_fkey FOREIGN KEY (document_id) REFERENCES public.document_readings(id);

ALTER TABLE ONLY tender.draft_sources
    ADD CONSTRAINT draft_sources_draft_id_fkey FOREIGN KEY (draft_id) REFERENCES tender.document_drafts(id);

ALTER TABLE ONLY tender.draft_sources
    ADD CONSTRAINT draft_sources_member_id_fkey FOREIGN KEY (member_id) REFERENCES public.document_package_members(id);

ALTER TABLE ONLY tender.prebid_queries
    ADD CONSTRAINT prebid_queries_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.prebid_queries
    ADD CONSTRAINT prebid_queries_draft_id_fkey FOREIGN KEY (draft_id) REFERENCES tender.document_drafts(id);

ALTER TABLE ONLY tender.prebid_queries
    ADD CONSTRAINT prebid_queries_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tender_capture_operations
    ADD CONSTRAINT tender_capture_operations_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tender_capture_operations
    ADD CONSTRAINT tender_capture_operations_tender_id_fkey FOREIGN KEY (tender_id) REFERENCES tender.tender_fms_intakes(id);

ALTER TABLE ONLY tender.tender_email_attempts
    ADD CONSTRAINT tender_email_attempts_job_id_fkey FOREIGN KEY (job_id) REFERENCES tender.tender_email_jobs(id);

ALTER TABLE ONLY tender.tender_email_jobs
    ADD CONSTRAINT tender_email_jobs_action_id_fkey FOREIGN KEY (action_id) REFERENCES tender.tender_source_actions(id);

ALTER TABLE ONLY tender.tender_email_jobs
    ADD CONSTRAINT tender_email_jobs_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tender_email_jobs
    ADD CONSTRAINT tender_email_jobs_recipient_id_fkey FOREIGN KEY (recipient_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tender_email_jobs
    ADD CONSTRAINT tender_email_jobs_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tender_fms_intakes
    ADD CONSTRAINT tender_fms_current_manager FOREIGN KEY (manager_assignment_id, id, owner_id, assignment_version) REFERENCES tender.tender_fms_manager_assignments(id, intake_id, manager_id, version) DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE ONLY tender.tender_fms_events
    ADD CONSTRAINT tender_fms_events_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tender_fms_events
    ADD CONSTRAINT tender_fms_events_document_id_fkey FOREIGN KEY (document_id) REFERENCES public.document_readings(id);

ALTER TABLE ONLY tender.tender_fms_events
    ADD CONSTRAINT tender_fms_events_intake_id_fkey FOREIGN KEY (intake_id) REFERENCES tender.tender_fms_intakes(id);

ALTER TABLE ONLY tender.tender_fms_intakes
    ADD CONSTRAINT tender_fms_intakes_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tender_fms_intakes
    ADD CONSTRAINT tender_fms_intakes_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tender_fms_intakes
    ADD CONSTRAINT tender_fms_intakes_receipt_id_fkey FOREIGN KEY (receipt_id) REFERENCES public.document_package_receipts(id);

ALTER TABLE ONLY tender.tender_fms_intakes
    ADD CONSTRAINT tender_fms_intakes_sales_creator_id_fkey FOREIGN KEY (sales_creator_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tender_fms_intakes
    ADD CONSTRAINT tender_fms_intakes_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tender_fms_manager_assignments
    ADD CONSTRAINT tender_fms_manager_assignments_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tender_fms_manager_assignments
    ADD CONSTRAINT tender_fms_manager_assignments_intake_id_fkey FOREIGN KEY (intake_id) REFERENCES tender.tender_fms_intakes(id) DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE ONLY tender.tender_fms_manager_assignments
    ADD CONSTRAINT tender_fms_manager_assignments_manager_id_fkey FOREIGN KEY (manager_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tender_fms_owner_changes
    ADD CONSTRAINT tender_fms_owner_changes_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tender_fms_owner_changes
    ADD CONSTRAINT tender_fms_owner_changes_intake_id_fkey FOREIGN KEY (intake_id) REFERENCES tender.tender_fms_intakes(id);

ALTER TABLE ONLY tender.tender_fms_owner_changes
    ADD CONSTRAINT tender_fms_owner_changes_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tender_fms_owner_changes
    ADD CONSTRAINT tender_fms_owner_changes_previous_owner_id_fkey FOREIGN KEY (previous_owner_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tender_fms_participant_grants
    ADD CONSTRAINT tender_fms_participant_grants_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tender_fms_participant_grants
    ADD CONSTRAINT tender_fms_participant_grants_intake_id_fkey FOREIGN KEY (intake_id) REFERENCES tender.tender_fms_intakes(id);

ALTER TABLE ONLY tender.tender_fms_participant_grants
    ADD CONSTRAINT tender_fms_participant_grants_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tender_fms_record_events
    ADD CONSTRAINT tender_fms_record_events_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tender_fms_record_events
    ADD CONSTRAINT tender_fms_record_events_intake_id_fkey FOREIGN KEY (intake_id) REFERENCES tender.tender_fms_intakes(id);

ALTER TABLE ONLY tender.tender_provision_operations
    ADD CONSTRAINT tender_provision_operations_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tender_provision_operations
    ADD CONSTRAINT tender_provision_operations_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.projects(id);

ALTER TABLE ONLY tender.tender_provision_operations
    ADD CONSTRAINT tender_provision_operations_tender_id_fkey FOREIGN KEY (tender_id) REFERENCES tender.tender_fms_intakes(id);

ALTER TABLE ONLY tender.tender_provision_operations
    ADD CONSTRAINT tender_provision_operations_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tender_source_actions
    ADD CONSTRAINT tender_source_action_notice FOREIGN KEY (notice_id) REFERENCES public.communication_notices(id);

ALTER TABLE ONLY tender.tender_source_actions
    ADD CONSTRAINT tender_source_actions_assignment_id_fkey FOREIGN KEY (assignment_id) REFERENCES tender.tender_fms_manager_assignments(id);

ALTER TABLE ONLY tender.tender_source_actions
    ADD CONSTRAINT tender_source_actions_assignment_id_tender_id_recipient_id_fkey FOREIGN KEY (assignment_id, tender_id, recipient_id, assignment_version) REFERENCES tender.tender_fms_manager_assignments(id, intake_id, manager_id, version);

ALTER TABLE ONLY tender.tender_source_actions
    ADD CONSTRAINT tender_source_actions_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tender_source_actions
    ADD CONSTRAINT tender_source_actions_recipient_id_fkey FOREIGN KEY (recipient_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tender_source_actions
    ADD CONSTRAINT tender_source_actions_source_event_id_fkey FOREIGN KEY (source_event_id) REFERENCES public.business_events(id) DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE ONLY tender.tender_source_actions
    ADD CONSTRAINT tender_source_actions_tender_id_fkey FOREIGN KEY (tender_id) REFERENCES tender.tender_fms_intakes(id);

ALTER TABLE ONLY tender.tender_source_actions
    ADD CONSTRAINT tender_source_actions_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tender_source_links
    ADD CONSTRAINT tender_source_links_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tender_source_links
    ADD CONSTRAINT tender_source_links_document_id_fkey FOREIGN KEY (document_id) REFERENCES public.document_readings(id);

ALTER TABLE ONLY tender.tender_source_links
    ADD CONSTRAINT tender_source_links_member_id_fkey FOREIGN KEY (member_id) REFERENCES public.document_package_members(id);

ALTER TABLE ONLY tender.tender_source_links
    ADD CONSTRAINT tender_source_links_tender_id_fkey FOREIGN KEY (tender_id) REFERENCES tender.tender_fms_intakes(id);

ALTER TABLE ONLY tender.tsc_cloud_calls
    ADD CONSTRAINT tsc_cloud_calls_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tsc_cloud_calls
    ADD CONSTRAINT tsc_cloud_calls_draft_id_fkey FOREIGN KEY (draft_id) REFERENCES tender.document_drafts(id);

ALTER TABLE ONLY tender.tsc_cloud_calls
    ADD CONSTRAINT tsc_cloud_calls_job_id_fkey FOREIGN KEY (job_id) REFERENCES tender.tsc_jobs(id);

ALTER TABLE ONLY tender.tsc_document_text
    ADD CONSTRAINT tsc_document_text_member_id_fkey FOREIGN KEY (member_id) REFERENCES public.document_package_members(id);

ALTER TABLE ONLY tender.tsc_eligibility_verdicts
    ADD CONSTRAINT tsc_eligibility_verdicts_draft_id_fkey FOREIGN KEY (draft_id) REFERENCES tender.document_drafts(id);

ALTER TABLE ONLY tender.tsc_eligibility_verdicts
    ADD CONSTRAINT tsc_eligibility_verdicts_job_id_fkey FOREIGN KEY (job_id) REFERENCES tender.tsc_workspaces(id);

ALTER TABLE ONLY tender.tsc_eligibility_verdicts
    ADD CONSTRAINT tsc_eligibility_verdicts_verified_by_id_fkey FOREIGN KEY (verified_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tsc_events
    ADD CONSTRAINT tsc_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tsc_events
    ADD CONSTRAINT tsc_events_job_id_fkey FOREIGN KEY (job_id) REFERENCES tender.tsc_jobs(id);

ALTER TABLE ONLY tender.tsc_format_events
    ADD CONSTRAINT tsc_format_events_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tsc_format_events
    ADD CONSTRAINT tsc_format_events_format_id_fkey FOREIGN KEY (format_id) REFERENCES tender.tsc_formats(id);

ALTER TABLE ONLY tender.tsc_format_questions
    ADD CONSTRAINT tsc_format_questions_format_id_fkey FOREIGN KEY (format_id) REFERENCES tender.tsc_formats(id);

ALTER TABLE ONLY tender.tsc_formats
    ADD CONSTRAINT tsc_formats_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tsc_formats
    ADD CONSTRAINT tsc_formats_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tsc_jobs
    ADD CONSTRAINT tsc_jobs_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tsc_jobs
    ADD CONSTRAINT tsc_jobs_draft_id_fkey FOREIGN KEY (draft_id) REFERENCES tender.document_drafts(id);

ALTER TABLE ONLY tender.tsc_jobs
    ADD CONSTRAINT tsc_jobs_format_id_fkey FOREIGN KEY (format_id) REFERENCES tender.tsc_formats(id);

ALTER TABLE ONLY tender.tsc_model_calls
    ADD CONSTRAINT tsc_model_calls_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tsc_model_calls
    ADD CONSTRAINT tsc_model_calls_job_id_draft_id_fkey FOREIGN KEY (job_id, draft_id) REFERENCES tender.tsc_workspaces(id, draft_id);

ALTER TABLE ONLY tender.tsc_model_calls
    ADD CONSTRAINT tsc_model_calls_task_id_fkey FOREIGN KEY (task_id) REFERENCES tender.tsc_workspace_tasks(id);

ALTER TABLE ONLY tender.tsc_model_responses
    ADD CONSTRAINT tsc_model_responses_call_id_job_id_draft_id_fkey FOREIGN KEY (call_id, job_id, draft_id) REFERENCES tender.tsc_model_calls(id, job_id, draft_id);

ALTER TABLE ONLY tender.tsc_workspace_events
    ADD CONSTRAINT tsc_workspace_events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tsc_workspace_events
    ADD CONSTRAINT tsc_workspace_events_job_id_fkey FOREIGN KEY (job_id) REFERENCES tender.tsc_workspaces(id);

ALTER TABLE ONLY tender.tsc_workspace_tasks
    ADD CONSTRAINT tsc_workspace_tasks_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tsc_workspace_tasks
    ADD CONSTRAINT tsc_workspace_tasks_job_id_draft_id_fkey FOREIGN KEY (job_id, draft_id) REFERENCES tender.tsc_workspaces(id, draft_id);

ALTER TABLE ONLY tender.tsc_workspaces
    ADD CONSTRAINT tsc_workspaces_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES auth.users(id);

ALTER TABLE ONLY tender.tsc_workspaces
    ADD CONSTRAINT tsc_workspaces_draft_id_fkey FOREIGN KEY (draft_id) REFERENCES tender.document_drafts(id);

ALTER TABLE ONLY tender.vendor_quotations
    ADD CONSTRAINT vendor_quotations_bid_item_id_fkey FOREIGN KEY (bid_item_id) REFERENCES tender.bid_items(id) ON DELETE CASCADE;

ALTER TABLE ONLY tender.vendor_quotations
    ADD CONSTRAINT vendor_quotations_draft_id_fkey FOREIGN KEY (draft_id) REFERENCES tender.document_drafts(id);

ALTER TABLE ONLY tender.vendor_quotations
    ADD CONSTRAINT vendor_quotations_vendor_id_fkey FOREIGN KEY (vendor_id) REFERENCES public.suppliers(id);

COMMIT;
