CREATE TABLE tender_fms_owner_changes (
 id uuid PRIMARY KEY,intake_id uuid NOT NULL REFERENCES tender_fms_intakes(id),
 previous_owner_id uuid NOT NULL REFERENCES users(id),owner_id uuid NOT NULL REFERENCES users(id),
 version integer NOT NULL,reason text NOT NULL CHECK(length(reason) BETWEEN 10 AND 1000),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid NOT NULL REFERENCES users(id),
 UNIQUE(intake_id,version),CHECK(previous_owner_id<>owner_id)
);
CREATE TRIGGER tender_fms_owner_changes_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON tender_fms_owner_changes
FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
