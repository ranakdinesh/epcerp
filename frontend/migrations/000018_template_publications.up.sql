BEGIN;
ALTER TABLE configuration_templates DROP CONSTRAINT configuration_templates_template_name_revision_key;
CREATE TABLE template_publications (
 template_id uuid PRIMARY KEY REFERENCES configuration_templates(id),
 previous_template_id uuid REFERENCES configuration_templates(id),
 reason text NOT NULL CHECK(length(trim(reason)) BETWEEN 1 AND 1000),
 before_definition jsonb NOT NULL,
 after_definition jsonb NOT NULL,
 is_active boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 created_by_id uuid REFERENCES users(id),
 updated_by_id uuid REFERENCES users(id)
);
CREATE TRIGGER immutable_publication BEFORE UPDATE OR DELETE ON template_publications FOR EACH ROW EXECUTE FUNCTION protect_template_revision();
COMMIT;
