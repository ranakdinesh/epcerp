BEGIN;
ALTER TABLE tender_fms_intakes
 DROP CONSTRAINT tender_fms_intakes_owner_id_initial_sha256_key,
 ALTER COLUMN initial_sha256 DROP NOT NULL,
 ADD COLUMN record_kind text NOT NULL DEFAULT 'legacy' CHECK(record_kind IN ('legacy','named')),
 ADD COLUMN legacy_title text,
 ADD COLUMN sales_creator_id uuid REFERENCES users(id),
 ADD COLUMN business_version bigint NOT NULL DEFAULT 1 CHECK(business_version BETWEEN 1 AND 9007199254740991),
 ADD COLUMN content_revision bigint NOT NULL DEFAULT 1 CHECK(content_revision BETWEEN 1 AND 9007199254740991),
 ADD COLUMN assignment_version bigint NOT NULL DEFAULT 1 CHECK(assignment_version BETWEEN 1 AND 9007199254740991),
 ADD COLUMN manager_assignment_id uuid;
UPDATE tender_fms_intakes SET legacy_title=title;
CREATE UNIQUE INDEX tender_fms_legacy_fingerprint ON tender_fms_intakes(owner_id,initial_sha256) WHERE record_kind='legacy';
ALTER TABLE tender_fms_intakes ADD CONSTRAINT tender_fms_identity_kind CHECK(
 (record_kind='legacy' AND initial_sha256 IS NOT NULL AND sales_creator_id IS NULL AND manager_assignment_id IS NULL)
 OR (record_kind='named' AND length(btrim(title)) BETWEEN 2 AND 255 AND sales_creator_id IS NOT NULL AND manager_assignment_id IS NOT NULL));
CREATE TABLE tender_fms_manager_assignments (
 id uuid PRIMARY KEY, intake_id uuid NOT NULL REFERENCES tender_fms_intakes(id) DEFERRABLE INITIALLY DEFERRED,
 manager_id uuid NOT NULL REFERENCES users(id), version bigint NOT NULL CHECK(version BETWEEN 1 AND 9007199254740991),
 reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 10 AND 1000),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(), created_by_id uuid NOT NULL REFERENCES users(id),
 UNIQUE(intake_id,version), UNIQUE(id,intake_id,manager_id,version)
);
ALTER TABLE tender_fms_intakes ADD CONSTRAINT tender_fms_current_manager
 FOREIGN KEY(manager_assignment_id,id,owner_id,assignment_version)
 REFERENCES tender_fms_manager_assignments(id,intake_id,manager_id,version) DEFERRABLE INITIALLY DEFERRED;
CREATE TRIGGER tender_fms_manager_assignments_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON tender_fms_manager_assignments
 FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
CREATE TABLE tender_fms_record_events (
 id uuid PRIMARY KEY, intake_id uuid NOT NULL REFERENCES tender_fms_intakes(id),
 business_version bigint NOT NULL CHECK(business_version BETWEEN 1 AND 9007199254740991),
 action text NOT NULL CHECK(action IN ('named_record_created','metadata_updated','manager_assigned')),
 actor_id uuid NOT NULL REFERENCES users(id), created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(intake_id,business_version)
);
CREATE TRIGGER tender_fms_record_events_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON tender_fms_record_events
 FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
CREATE INDEX tender_fms_sales_lookup ON tender_fms_intakes(sales_creator_id,id) WHERE record_kind='named';
CREATE TABLE tender_fms_participant_grants (
 id uuid PRIMARY KEY, intake_id uuid NOT NULL REFERENCES tender_fms_intakes(id), user_id uuid NOT NULL REFERENCES users(id),
 version bigint NOT NULL CHECK(version BETWEEN 1 AND 9007199254740991), is_active boolean NOT NULL,
 valid_until timestamptz, reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 10 AND 1000),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(), created_by_id uuid NOT NULL REFERENCES users(id),
 UNIQUE(intake_id,user_id,version)
);
CREATE TRIGGER tender_fms_participant_grants_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON tender_fms_participant_grants
 FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
COMMIT;
