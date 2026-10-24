BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM organisation_director_designation) OR EXISTS(SELECT 1 FROM organisation_director_events) THEN RAISE EXCEPTION 'Refuse to remove populated Director designation evidence';END IF;END $$;
DROP TABLE organisation_director_events,organisation_director_designation;
DROP FUNCTION protect_organisation_director_identity(),protect_organisation_director_evidence();
COMMIT;
