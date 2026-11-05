BEGIN;
SET LOCAL lock_timeout = '10s';
CREATE SCHEMA auth;
ALTER TABLE public.users SET SCHEMA auth;
ALTER TABLE public.roles SET SCHEMA auth;
ALTER TABLE public.permissions SET SCHEMA auth;
ALTER TABLE public.user_roles SET SCHEMA auth;
ALTER TABLE public.role_permissions SET SCHEMA auth;
ALTER TABLE public.refresh_tokens SET SCHEMA auth;
ALTER TABLE public.user_activity SET SCHEMA auth;
ALTER TABLE public.user_profile SET SCHEMA auth;
ALTER TABLE public.auth_password_recovery SET SCHEMA auth;
ALTER TABLE public.auth_credential_emails SET SCHEMA auth;
ALTER TABLE public.auth_credential_changes SET SCHEMA auth;
ALTER TABLE public.auth_departments SET SCHEMA auth;
ALTER TABLE public.auth_reporting_lines SET SCHEMA auth;
ALTER TABLE public.auth_organisation_changes SET SCHEMA auth;
ALTER TABLE public.auth_offboarding_cases SET SCHEMA auth;
ALTER TABLE public.auth_offboarding_changes SET SCHEMA auth;
ALTER TABLE public.auth_personal_contact_changes SET SCHEMA auth;
ALTER TABLE public.auth_username_changes SET SCHEMA auth;
ALTER TABLE public.auth_bootstrap_identity SET SCHEMA auth;
ALTER FUNCTION public.reject_auth_credential_change_mutation() SET SCHEMA auth;
ALTER FUNCTION public.reject_auth_organisation_change_mutation() SET SCHEMA auth;
ALTER FUNCTION public.guard_auth_reporting_line() SET SCHEMA auth;
CREATE OR REPLACE FUNCTION auth.guard_auth_reporting_line() RETURNS trigger LANGUAGE plpgsql AS $$
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
-- Preserve the inbox guard while its Auth lookup moves namespaces.
CREATE OR REPLACE FUNCTION public.protect_communication_notice() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF TG_OP='INSERT' THEN
  IF NEW.version<>1 OR NEW.read_at IS NOT NULL OR NEW.acknowledged_at IS NOT NULL OR NEW.created_at<>NEW.updated_at OR NEW.created_by_id IS DISTINCT FROM NEW.updated_by_id THEN RAISE EXCEPTION 'New notice must be unread with source attribution';END IF;
  IF NEW.notice_kind='director' THEN
  IF NEW.project_id IS NULL THEN RAISE EXCEPTION 'Director notice requires project'; END IF;
  IF NOT EXISTS(SELECT 1 FROM business_events e WHERE e.id=NEW.source_event_id AND e.source_module IN('finance_approvals','finance_accounts_acceptance') AND e.entity_type='financial_decision' AND e.entity_id=NEW.decision_id AND e.entity_version=NEW.source_version AND e.action=NEW.decision_action AND e.actor_id=NEW.created_by_id AND e.actor_kind='user' AND e.scope_kind='project' AND e.scope_id=NEW.project_id AND e.visibility='restricted' AND e.occurred_at=NEW.created_at) THEN RAISE EXCEPTION 'Notice source must be its recorded financial decision';END IF;
  IF NOT EXISTS(SELECT 1 FROM organisation_director_designation d JOIN auth.users u ON u.id=d.user_id WHERE d.id=NEW.director_designation_id AND d.version=NEW.director_version AND d.user_id=NEW.recipient_id AND d.is_active AND u.is_active) THEN RAISE EXCEPTION 'Notice requires the effective Director';END IF;
  ELSIF NEW.notice_kind='tender_manager' THEN
   IF NEW.project_id IS NOT NULL OR NEW.director_designation_id IS NOT NULL OR NEW.director_version<>0 OR NEW.source_action_id IS NULL OR NEW.title='' OR NOT EXISTS(
    SELECT 1 FROM business_events e WHERE e.id=NEW.source_event_id AND e.source_module='tender_fms' AND e.entity_type='tender_assignment'
    AND e.entity_id=NEW.decision_id AND e.entity_version=NEW.source_version AND e.action='manager_assigned' AND NEW.decision_action='assigned'
    AND e.actor_id=NEW.created_by_id AND e.actor_kind='user' AND e.scope_kind='platform' AND e.scope_id IS NULL AND e.visibility='restricted' AND e.occurred_at=NEW.created_at
   ) THEN RAISE EXCEPTION 'Tender Manager notice requires its recorded assignment source';END IF;
  ELSIF NEW.notice_kind='supplier_review' THEN
   IF NEW.project_id IS NOT NULL OR NEW.director_designation_id IS NOT NULL OR NEW.director_version<>0 OR NEW.title='' OR NOT EXISTS(
    SELECT 1 FROM business_events e JOIN supplier_review_assignments a ON a.id=e.entity_id
    WHERE e.id=NEW.source_event_id AND e.source_module='supplier_review_assignments' AND e.entity_type='supplier_review_assignment'
    AND e.entity_id=NEW.decision_id AND e.entity_version=NEW.source_version AND e.action='assigned' AND NEW.decision_action='assigned'
    AND e.actor_id=NEW.created_by_id AND e.actor_kind='user' AND e.scope_kind='platform' AND e.scope_id IS NULL
    AND e.visibility='restricted' AND e.occurred_at=NEW.created_at AND a.version=NEW.source_version AND a.user_id=NEW.recipient_id AND a.is_active
    AND NEW.link='/dashboard?screen=suppliers&supplier_id='||a.supplier_id::text
   ) THEN RAISE EXCEPTION 'Supplier review notice requires current recorded assignment';END IF;
  ELSE
   IF NEW.project_id IS NULL THEN RAISE EXCEPTION 'Quotation notice requires project'; END IF;
   IF NEW.director_designation_id IS NOT NULL OR NEW.director_version<>0 OR NEW.title='' OR NEW.link !~ '^/dashboard\?' OR NOT EXISTS(SELECT 1 FROM business_events e WHERE e.id=NEW.source_event_id AND e.source_module='quotation_approvals' AND e.entity_type='quotation_approval_set' AND e.entity_id=NEW.decision_id AND e.entity_version=NEW.source_version AND e.action=NEW.decision_action AND e.action IN('submit','accounts_decision') AND e.actor_id=NEW.created_by_id AND e.scope_kind='project' AND e.scope_id=NEW.project_id AND e.visibility='restricted' AND e.occurred_at=NEW.created_at) THEN RAISE EXCEPTION 'Addressed quotation notice requires its recorded source';END IF;
  END IF;
 ELSE
  IF ROW(NEW.source_action_id,NEW.notice_kind,NEW.title,NEW.link,NEW.id,NEW.source_event_id,NEW.recipient_id,NEW.director_designation_id,NEW.director_version,NEW.decision_id,NEW.project_id,NEW.source_version,NEW.decision_action,NEW.is_active,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.source_action_id,OLD.notice_kind,OLD.title,OLD.link,OLD.id,OLD.source_event_id,OLD.recipient_id,OLD.director_designation_id,OLD.director_version,OLD.decision_id,OLD.project_id,OLD.source_version,OLD.decision_action,OLD.is_active,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Notice identity and addressed source are immutable';END IF;
  IF NEW.version<>OLD.version+1 OR NEW.updated_by_id IS DISTINCT FROM NEW.recipient_id OR NEW.updated_at<OLD.updated_at OR (OLD.read_at IS NOT NULL AND NEW.read_at IS DISTINCT FROM OLD.read_at) OR (OLD.acknowledged_at IS NOT NULL AND NEW.acknowledged_at IS DISTINCT FROM OLD.acknowledged_at) OR ROW(NEW.read_at,NEW.acknowledged_at) IS NOT DISTINCT FROM ROW(OLD.read_at,OLD.acknowledged_at) THEN RAISE EXCEPTION 'Notice receipts must advance monotonically for the recipient';END IF;
 END IF;RETURN NEW;END $$;
COMMIT;
