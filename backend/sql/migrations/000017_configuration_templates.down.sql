BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM configuration_templates) THEN RAISE EXCEPTION 'Cannot discard published template evidence'; END IF; END $$;
DROP TABLE template_options,template_fields,configuration_templates;
DROP FUNCTION protect_template_revision();
COMMIT;
