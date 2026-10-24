BEGIN;
CREATE TABLE manufacturer_imports(source_id uuid PRIMARY KEY,go_id uuid NOT NULL UNIQUE REFERENCES manufacturers(id),source_snapshot jsonb NOT NULL,source_fingerprint text NOT NULL,is_active boolean NOT NULL DEFAULT true,created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id));
CREATE FUNCTION protect_manufacturer_evidence() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Manufacturer audit and import evidence is immutable'; END; $$;
CREATE TRIGGER immutable_manufacturer_import BEFORE UPDATE OR DELETE ON manufacturer_imports FOR EACH ROW EXECUTE FUNCTION protect_manufacturer_evidence();
CREATE TRIGGER immutable_manufacturer_event BEFORE UPDATE OR DELETE ON manufacturers_events FOR EACH ROW EXECUTE FUNCTION protect_manufacturer_evidence();
COMMIT;
