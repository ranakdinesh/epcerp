BEGIN;
CREATE TABLE catalogue_items (
 id uuid PRIMARY KEY, configuration_id uuid NOT NULL REFERENCES configuration_templates(id), manufacturer_id uuid NOT NULL REFERENCES manufacturers(id), category_id uuid NOT NULL REFERENCES categories(id),
 model varchar(120) NOT NULL, specifications jsonb NOT NULL, snapshot jsonb NOT NULL, fingerprint varchar(64) NOT NULL UNIQUE, engineering_key varchar(64) NOT NULL,
 description text NOT NULL, sku varchar(1000) NOT NULL UNIQUE, serial integer NOT NULL CHECK(serial BETWEEN 1 AND 999),
 status varchar(20) NOT NULL CHECK(status IN ('approved','retired')), version integer NOT NULL DEFAULT 1 CHECK(version>0),is_active boolean NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id),
 UNIQUE(category_id,serial),CHECK(is_active=(status='approved'))
);
CREATE INDEX catalogue_items_engineering ON catalogue_items(engineering_key);
CREATE TABLE catalogue_item_events (
 id uuid PRIMARY KEY,subject_id uuid NOT NULL REFERENCES catalogue_items(id),action varchar(40) NOT NULL,reason varchar(2000) NOT NULL,before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id)
);
CREATE FUNCTION protect_catalogue_item_identity() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF ROW(NEW.configuration_id,NEW.manufacturer_id,NEW.category_id,NEW.model,NEW.specifications,NEW.snapshot,NEW.fingerprint,NEW.engineering_key,NEW.description,NEW.sku,NEW.serial,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.configuration_id,OLD.manufacturer_id,OLD.category_id,OLD.model,OLD.specifications,OLD.snapshot,OLD.fingerprint,OLD.engineering_key,OLD.description,OLD.sku,OLD.serial,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Issued item identity is immutable'; END IF; RETURN NEW; END $$;
CREATE TRIGGER catalogue_item_identity BEFORE UPDATE ON catalogue_items FOR EACH ROW EXECUTE FUNCTION protect_catalogue_item_identity();
CREATE FUNCTION protect_catalogue_item_evidence() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Item history and issued items cannot be deleted or rewritten'; END $$;
CREATE TRIGGER catalogue_item_no_delete BEFORE DELETE ON catalogue_items FOR EACH ROW EXECUTE FUNCTION protect_catalogue_item_evidence();
CREATE TRIGGER catalogue_item_events_immutable BEFORE UPDATE OR DELETE ON catalogue_item_events FOR EACH ROW EXECUTE FUNCTION protect_catalogue_item_evidence();
COMMIT;
