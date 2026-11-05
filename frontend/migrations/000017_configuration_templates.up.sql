BEGIN;
CREATE TABLE configuration_templates (
 id uuid PRIMARY KEY, subcategory_id uuid NOT NULL REFERENCES category_subcategories(id),
 template_name varchar(255) NOT NULL, model_required boolean NOT NULL, revision integer NOT NULL CHECK(revision>0),
 is_active boolean NOT NULL DEFAULT true, version integer NOT NULL DEFAULT 1 CHECK(version>0),
 created_at timestamptz NOT NULL DEFAULT now(), created_by_id uuid REFERENCES users(id),
 updated_at timestamptz NOT NULL DEFAULT now(), updated_by_id uuid REFERENCES users(id),
 legacy_definition jsonb NOT NULL, source_fingerprint text NOT NULL,
 UNIQUE(subcategory_id,revision), UNIQUE(template_name,revision)
);
CREATE TABLE template_fields (
 id uuid PRIMARY KEY, template_id uuid NOT NULL REFERENCES configuration_templates(id),
 field_code varchar(32) NOT NULL, field_name varchar(255) NOT NULL, input_type text NOT NULL CHECK(input_type='dropdown'),
 mandatory boolean NOT NULL, allow_na boolean NOT NULL, use_in_description boolean NOT NULL,
 description_order integer NOT NULL CHECK(description_order>=0), description_prefix varchar(128) NOT NULL, description_suffix varchar(128) NOT NULL,
 use_in_duplicate_check boolean NOT NULL, sort_order integer NOT NULL CHECK(sort_order>0),
 is_active boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT now(), created_by_id uuid REFERENCES users(id),
 updated_at timestamptz NOT NULL DEFAULT now(), updated_by_id uuid REFERENCES users(id),
 CHECK((use_in_description AND description_order>0) OR (NOT use_in_description AND description_order=0)),
 UNIQUE(template_id,field_code), UNIQUE(template_id,sort_order)
);
CREATE UNIQUE INDEX template_description_order ON template_fields(template_id,description_order) WHERE use_in_description;
CREATE TABLE template_options (
 id uuid PRIMARY KEY, field_id uuid NOT NULL REFERENCES template_fields(id), option_code varchar(32) NOT NULL, option_label varchar(255) NOT NULL,
 sort_order integer NOT NULL CHECK(sort_order>0), is_active boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT now(), created_by_id uuid REFERENCES users(id),
 updated_at timestamptz NOT NULL DEFAULT now(), updated_by_id uuid REFERENCES users(id),
 UNIQUE(field_id,option_code),UNIQUE(field_id,sort_order)
);
CREATE FUNCTION protect_template_revision() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Published template revisions are immutable; publish a new revision'; END; $$;
CREATE TRIGGER immutable_template BEFORE UPDATE OR DELETE ON configuration_templates FOR EACH ROW EXECUTE FUNCTION protect_template_revision();
CREATE TRIGGER immutable_template_field BEFORE UPDATE OR DELETE ON template_fields FOR EACH ROW EXECUTE FUNCTION protect_template_revision();
CREATE TRIGGER immutable_template_option BEFORE UPDATE OR DELETE ON template_options FOR EACH ROW EXECUTE FUNCTION protect_template_revision();
COMMIT;
