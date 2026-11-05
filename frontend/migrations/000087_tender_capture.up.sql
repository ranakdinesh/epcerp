BEGIN;
ALTER TABLE tender_fms_intakes ADD COLUMN receipt_id uuid UNIQUE REFERENCES document_package_receipts(id),ADD COLUMN external_reference text CHECK(length(external_reference)<=255),ADD COLUMN source_url text CHECK(length(source_url)<=2048);
CREATE TABLE tender_provision_operations (
 id uuid PRIMARY KEY, tender_id uuid NOT NULL UNIQUE REFERENCES tender_fms_intakes(id),
 state text NOT NULL DEFAULT 'pending' CHECK(state IN('pending','ready','blocked_configuration','failed_retryable','needs_reconciliation')),
 version bigint NOT NULL DEFAULT 1 CHECK(version>0), blocker_code text NOT NULL DEFAULT 'project_provider_pending',
 project_id uuid UNIQUE REFERENCES projects(id), estimate_id uuid UNIQUE,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id),
 CHECK((state='ready')=(project_id IS NOT NULL AND estimate_id IS NOT NULL))
);
CREATE TABLE tender_capture_operations (
 actor_id uuid NOT NULL REFERENCES users(id),command text NOT NULL,context_key text NOT NULL,idempotency_key uuid NOT NULL,
 fingerprint text NOT NULL CHECK(fingerprint ~ '^[0-9a-f]{64}$'),tender_id uuid NOT NULL REFERENCES tender_fms_intakes(id),created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(actor_id,command,context_key,idempotency_key)
);
CREATE TABLE tender_source_links (
 member_id uuid PRIMARY KEY REFERENCES document_package_members(id),tender_id uuid NOT NULL REFERENCES tender_fms_intakes(id),document_id uuid NOT NULL REFERENCES document_readings(id),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id)
);
CREATE TABLE tender_source_actions (
 id uuid PRIMARY KEY,tender_id uuid NOT NULL REFERENCES tender_fms_intakes(id),assignment_id uuid NOT NULL REFERENCES tender_fms_manager_assignments(id),
 recipient_id uuid NOT NULL REFERENCES users(id),source_event_id uuid NOT NULL UNIQUE REFERENCES business_events(id) DEFERRABLE INITIALLY DEFERRED,assignment_version bigint NOT NULL CHECK(assignment_version>0),
 notice_id uuid, kind text NOT NULL DEFAULT 'manager_evaluation' CHECK(kind='manager_evaluation'),state text NOT NULL DEFAULT 'pending' CHECK(state IN('pending','cancelled_obsolete','completed')),
 is_active boolean NOT NULL DEFAULT true,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id),
 UNIQUE(tender_id,kind,assignment_version), UNIQUE(id,source_event_id,recipient_id,assignment_version),FOREIGN KEY(assignment_id,tender_id,recipient_id,assignment_version) REFERENCES tender_fms_manager_assignments(id,intake_id,manager_id,version)
);
CREATE TABLE tender_email_jobs (
 id uuid PRIMARY KEY,action_id uuid NOT NULL UNIQUE REFERENCES tender_source_actions(id),recipient_id uuid NOT NULL REFERENCES users(id),
 state text NOT NULL DEFAULT 'queued' CHECK(state IN('queued','attempting','accepted_by_provider','failed','cancelled_obsolete')),version bigint NOT NULL DEFAULT 1 CHECK(version>0),
 attempts integer NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 5),claim_id uuid,lease_until timestamptz,next_attempt_at timestamptz NOT NULL DEFAULT clock_timestamp(),error_code text,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id)
);
CREATE TABLE tender_email_attempts (
 id uuid PRIMARY KEY,job_id uuid NOT NULL REFERENCES tender_email_jobs(id),claim_id uuid NOT NULL, outcome text NOT NULL CHECK(outcome IN('claimed','accepted_by_provider','failed','outcome_unknown','cancelled_obsolete')),
 error_code text,created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TRIGGER tender_capture_operations_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON tender_capture_operations FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
CREATE TRIGGER tender_source_links_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON tender_source_links FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
CREATE TRIGGER tender_email_attempts_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON tender_email_attempts FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
COMMIT;
