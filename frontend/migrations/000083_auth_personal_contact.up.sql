CREATE TABLE auth_personal_contact_changes (
 id uuid PRIMARY KEY, user_id uuid NOT NULL REFERENCES users(id), version bigint NOT NULL CHECK(version>0),
 reason text NOT NULL CHECK(length(reason) BETWEEN 10 AND 1000), before_snapshot jsonb NOT NULL, after_snapshot jsonb NOT NULL,
 actor_id uuid NOT NULL REFERENCES users(id), created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(user_id,version), CHECK(actor_id=user_id)
);
CREATE TRIGGER auth_personal_contact_changes_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON auth_personal_contact_changes
FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
