BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM communication_notices) OR EXISTS(SELECT 1 FROM communication_notice_events) THEN RAISE EXCEPTION 'Refuse to remove populated mandatory notice evidence';END IF;END $$;
DROP TABLE communication_notice_events,communication_notices;
DROP FUNCTION protect_communication_notice();
COMMIT;
