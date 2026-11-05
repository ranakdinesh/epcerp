BEGIN;
CREATE TABLE organisation_director_designation(
 slot boolean PRIMARY KEY DEFAULT true CHECK(slot),id uuid NOT NULL UNIQUE,user_id uuid NOT NULL REFERENCES users(id),version integer NOT NULL CHECK(version>0),is_active boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id)
);
CREATE TABLE organisation_director_events(
 id uuid PRIMARY KEY,designation_id uuid NOT NULL REFERENCES organisation_director_designation(id),action varchar(30) NOT NULL CHECK(action IN('designate','change','revoke','reactivate')),reason varchar(2000) NOT NULL CHECK(length(trim(reason))>0),
 before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id)
);
CREATE INDEX organisation_director_history ON organisation_director_events(created_at,id);
CREATE FUNCTION protect_organisation_director_evidence() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Director designation evidence cannot be deleted or rewritten';END $$;
CREATE TRIGGER organisation_director_no_delete BEFORE DELETE ON organisation_director_designation FOR EACH ROW EXECUTE FUNCTION protect_organisation_director_evidence();
CREATE TRIGGER organisation_director_no_truncate BEFORE TRUNCATE ON organisation_director_designation FOR EACH STATEMENT EXECUTE FUNCTION protect_organisation_director_evidence();
CREATE TRIGGER organisation_director_events_immutable BEFORE UPDATE OR DELETE ON organisation_director_events FOR EACH ROW EXECUTE FUNCTION protect_organisation_director_evidence();
CREATE TRIGGER organisation_director_events_no_truncate BEFORE TRUNCATE ON organisation_director_events FOR EACH STATEMENT EXECUTE FUNCTION protect_organisation_director_evidence();
CREATE FUNCTION protect_organisation_director_identity() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF ROW(NEW.slot,NEW.id,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.slot,OLD.id,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Director designation identity is immutable';END IF;
 IF NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'Director designation requires the next version';END IF;RETURN NEW;END $$;
CREATE TRIGGER organisation_director_identity BEFORE UPDATE ON organisation_director_designation FOR EACH ROW EXECUTE FUNCTION protect_organisation_director_identity();
COMMIT;
