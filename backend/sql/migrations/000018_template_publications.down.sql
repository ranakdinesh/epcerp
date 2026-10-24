BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM template_publications) THEN RAISE EXCEPTION 'Cannot remove populated template publication evidence'; END IF; END $$;
DROP TABLE template_publications;
ALTER TABLE configuration_templates ADD UNIQUE(template_name,revision);
COMMIT;
