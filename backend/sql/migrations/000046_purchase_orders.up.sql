BEGIN;
CREATE TABLE purchase_orders(
 id uuid PRIMARY KEY,project_id uuid NOT NULL REFERENCES projects(id),package_id uuid NOT NULL UNIQUE REFERENCES purchase_work_packages(id),
 sourcing_id uuid NOT NULL REFERENCES purchase_sourcing_cases(id),technical_id uuid NOT NULL REFERENCES purchase_technical_cases(id),
 version integer NOT NULL CHECK(version>0),state varchar(20) NOT NULL CHECK(state IN('draft','released')),number varchar(80) UNIQUE,
 source_snapshot jsonb NOT NULL CHECK(jsonb_typeof(source_snapshot)='object'),data jsonb NOT NULL CHECK(jsonb_typeof(data)='object'),
 is_active boolean NOT NULL DEFAULT true,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id),UNIQUE(id,project_id),
 CHECK((state='draft' AND number IS NULL)OR(state='released' AND number IS NOT NULL))
);
CREATE TABLE purchase_order_events(
 id uuid PRIMARY KEY,subject_id uuid NOT NULL REFERENCES purchase_orders(id),action varchar(30) NOT NULL CHECK(action IN('prepare','release')),
 reason text NOT NULL CHECK(length(trim(reason))BETWEEN 1 AND 2000),before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id)
);
CREATE INDEX purchase_order_history ON purchase_order_events(subject_id,created_at DESC);

CREATE TABLE purchase_order_document_events(id uuid PRIMARY KEY,order_id uuid NOT NULL REFERENCES purchase_orders(id),order_version integer NOT NULL CHECK(order_version>0),created_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id));
-- Technical sequence lifecycle; increment and release/audit share one transaction.
CREATE TABLE purchase_order_sequences(fiscal_year integer PRIMARY KEY CHECK(fiscal_year BETWEEN 1 AND 9999),last_value bigint NOT NULL CHECK(last_value BETWEEN 1 AND 999999999));
CREATE FUNCTION protect_purchase_order() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_sourcing_cases s WHERE s.id=NEW.sourcing_id AND s.project_id=NEW.project_id AND s.package_id=NEW.package_id AND s.state='released' AND s.is_active AND NEW.source_snapshot->>'sourcing_id'=s.id::text AND NEW.source_snapshot->>'sourcing_version'=s.version::text AND NEW.source_snapshot->>'loi_reference'=s.data->>'loi_reference') THEN RAISE EXCEPTION 'Order requires its exact released project LOI';END IF;
 IF NOT EXISTS(SELECT 1 FROM purchase_technical_cases t WHERE t.id=NEW.technical_id AND t.project_id=NEW.project_id AND t.package_id=NEW.package_id AND t.is_active AND t.state IN('approved','approved_with_comments') AND NEW.source_snapshot->'technical'->>'case_id'=t.id::text AND NEW.source_snapshot->'technical'->>'version'=t.version::text AND NEW.source_snapshot->'technical'->>'revision'=t.revision::text) THEN RAISE EXCEPTION 'Order requires its exact approved technical revision';END IF;
 IF NEW.data->>'id' IS DISTINCT FROM NEW.id::text OR NEW.data->>'project_id' IS DISTINCT FROM NEW.project_id::text OR NEW.data->>'package_id' IS DISTINCT FROM NEW.package_id::text OR NEW.data->>'version' IS DISTINCT FROM NEW.version::text OR NEW.data->>'state' IS DISTINCT FROM NEW.state OR NEW.data->>'is_active' IS DISTINCT FROM NEW.is_active::text OR NEW.data->'source_snapshot' IS DISTINCT FROM NEW.source_snapshot OR coalesce(NEW.data->>'number','') IS DISTINCT FROM coalesce(NEW.number,'') THEN RAISE EXCEPTION 'Purchase order state projection mismatch';END IF;
 IF TG_OP='INSERT' AND (NEW.version<>1 OR NEW.state<>'draft') THEN RAISE EXCEPTION 'Orders begin as an owned draft';END IF;
 IF TG_OP='UPDATE' THEN
  IF OLD.state='released' THEN RAISE EXCEPTION 'Released purchase orders are immutable';END IF;
  IF NEW.id<>OLD.id OR NEW.project_id<>OLD.project_id OR NEW.package_id<>OLD.package_id OR NEW.sourcing_id<>OLD.sourcing_id OR NEW.technical_id<>OLD.technical_id OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'Order ownership and audit history must be retained';END IF;
  IF NEW.state='released' AND (NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot OR NEW.data->'details' IS DISTINCT FROM OLD.data->'details' OR NEW.data->>'released_by_id' IS DISTINCT FROM NEW.updated_by_id::text OR NEW.data->>'released_at' IS NULL) THEN RAISE EXCEPTION 'Release freezes prepared terms with authenticated attribution';END IF;
 END IF;
 RETURN NEW;END $$;
CREATE TRIGGER purchase_order_guard BEFORE INSERT OR UPDATE ON purchase_orders FOR EACH ROW EXECUTE FUNCTION protect_purchase_order();
CREATE FUNCTION protect_purchase_order_history() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Purchase orders and history cannot be erased';END $$;
CREATE TRIGGER purchase_order_no_delete BEFORE DELETE ON purchase_orders FOR EACH ROW EXECUTE FUNCTION protect_purchase_order_history();
CREATE TRIGGER purchase_order_no_truncate BEFORE TRUNCATE ON purchase_orders FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_order_history();
CREATE TRIGGER purchase_order_event_immutable BEFORE UPDATE OR DELETE ON purchase_order_events FOR EACH ROW EXECUTE FUNCTION protect_purchase_order_history();
CREATE TRIGGER purchase_order_event_no_truncate BEFORE TRUNCATE ON purchase_order_events FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_order_history();
CREATE FUNCTION check_purchase_order_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_order_events WHERE subject_id=NEW.id AND after_snapshot=NEW.data AND created_by_id IS NOT DISTINCT FROM NEW.updated_by_id AND created_at=NEW.updated_at AND (TG_OP='INSERT' OR before_snapshot=OLD.data)) THEN RAISE EXCEPTION 'Order mutation requires exact immutable audit';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_order_write_integrity AFTER INSERT OR UPDATE ON purchase_orders DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_order_write();
CREATE FUNCTION check_purchase_order_event() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE p uuid;BEGIN
 SELECT project_id INTO p FROM purchase_orders WHERE id=NEW.subject_id;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_orders' AND entity_type='purchase_order' AND entity_id=NEW.subject_id AND entity_version=(NEW.after_snapshot->>'version')::integer AND action=NEW.action AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted') THEN RAISE EXCEPTION 'Order audit requires immutable central project event';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_order_event_integrity AFTER INSERT ON purchase_order_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_order_event();

CREATE TRIGGER purchase_order_document_immutable BEFORE UPDATE OR DELETE ON purchase_order_document_events FOR EACH ROW EXECUTE FUNCTION protect_purchase_order_history();
CREATE TRIGGER purchase_order_document_no_truncate BEFORE TRUNCATE ON purchase_order_document_events FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_order_history();
CREATE FUNCTION check_purchase_order_document() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE p uuid;BEGIN
 SELECT project_id INTO p FROM purchase_orders WHERE id=NEW.order_id AND version=NEW.order_version;
 IF p IS NULL OR NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_orders' AND entity_type='purchase_order_document_event' AND entity_id=NEW.id AND entity_version=1 AND action='print_requested' AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted') THEN RAISE EXCEPTION 'Order document export requires current version and immutable project attribution';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_order_document_integrity AFTER INSERT ON purchase_order_document_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_order_document();
COMMIT;
