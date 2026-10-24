BEGIN;
CREATE TABLE purchase_acceptances(
 id uuid PRIMARY KEY,project_id uuid NOT NULL REFERENCES projects(id),package_id uuid NOT NULL UNIQUE REFERENCES purchase_work_packages(id),
 order_id uuid NOT NULL UNIQUE REFERENCES purchase_orders(id),order_version integer NOT NULL CHECK(order_version>1),
 version integer NOT NULL CHECK(version>0),status varchar(32) NOT NULL CHECK(status IN('pending','accepted','accepted_with_exceptions')),
 order_snapshot jsonb NOT NULL CHECK(jsonb_typeof(order_snapshot)='object'),data jsonb NOT NULL CHECK(jsonb_typeof(data)='object'),
 is_active boolean NOT NULL DEFAULT true,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id)
);
CREATE TABLE purchase_acceptance_events(
 id uuid PRIMARY KEY,subject_id uuid NOT NULL REFERENCES purchase_acceptances(id),action varchar(20) NOT NULL CHECK(action='record'),
 reason text NOT NULL CHECK(length(trim(reason)) BETWEEN 1 AND 2000),before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id)
);
CREATE INDEX purchase_acceptance_history ON purchase_acceptance_events(subject_id,created_at DESC);
CREATE FUNCTION protect_purchase_acceptance() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE o purchase_orders; expected jsonb; response jsonb;BEGIN
 SELECT * INTO o FROM purchase_orders WHERE id=NEW.order_id AND project_id=NEW.project_id AND package_id=NEW.package_id AND version=NEW.order_version AND state='released' AND is_active;
 IF o.id IS NULL THEN RAISE EXCEPTION 'Acceptance requires its actual released project PO';END IF;
 expected=jsonb_build_object('id',o.id,'project_id',o.project_id,'package_id',o.package_id,'version',o.version,'number',o.number,
 'date',o.data->'details'->>'date','supplier_id',o.source_snapshot->'supplier'->>'id','supplier_name',o.source_snapshot->'supplier'->>'name',
 'description',o.source_snapshot->>'description','quantity',o.source_snapshot->>'quantity','uom',o.source_snapshot->>'uom',
 'currency',o.source_snapshot->>'currency','total',o.source_snapshot->>'total','payment_terms',o.source_snapshot->>'payment_terms',
 'delivery_days',o.source_snapshot->'delivery_days','warranty',o.source_snapshot->>'warranty','released_at',o.data->'released_at','released_by_id',o.data->>'released_by_id');
 IF NEW.order_snapshot IS DISTINCT FROM expected THEN RAISE EXCEPTION 'Acceptance source must equal the immutable released PO';END IF;
 IF NEW.data->>'id' IS DISTINCT FROM NEW.id::text OR NEW.data->>'project_id' IS DISTINCT FROM NEW.project_id::text OR NEW.data->>'package_id' IS DISTINCT FROM NEW.package_id::text OR NEW.data->>'version' IS DISTINCT FROM NEW.version::text OR NEW.data->>'is_active' IS DISTINCT FROM NEW.is_active::text OR NEW.data->'order_snapshot' IS DISTINCT FROM NEW.order_snapshot OR NEW.data->'response'->>'status' IS DISTINCT FROM NEW.status THEN RAISE EXCEPTION 'Acceptance projection mismatch';END IF;
 response=NEW.data->'response';
 IF response->>'status'='pending' AND coalesce(response->>'accepted_date','')<>'' THEN RAISE EXCEPTION 'Pending response has no accepted date';END IF;
 IF response->>'status'<>'pending' AND (coalesce(response->>'accepted_date','')='' OR (response->>'accepted_date')::date<(o.data->'details'->>'date')::date) THEN RAISE EXCEPTION 'Acceptance requires a date on or after its PO';END IF;
 IF response->>'status'='accepted_with_exceptions' AND length(trim(coalesce(response->>'remarks','')))=0 THEN RAISE EXCEPTION 'Acceptance exceptions must be recorded';END IF;
 IF TG_OP='INSERT' AND NEW.version<>1 THEN RAISE EXCEPTION 'Acceptance begins at version one';END IF;
 IF TG_OP='UPDATE' AND (NEW.id<>OLD.id OR NEW.project_id<>OLD.project_id OR NEW.package_id<>OLD.package_id OR NEW.order_id<>OLD.order_id OR NEW.order_version<>OLD.order_version OR NEW.order_snapshot IS DISTINCT FROM OLD.order_snapshot OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1) THEN RAISE EXCEPTION 'Acceptance ownership/source/history must be retained';END IF;
 RETURN NEW;END $$;
CREATE TRIGGER purchase_acceptance_guard BEFORE INSERT OR UPDATE ON purchase_acceptances FOR EACH ROW EXECUTE FUNCTION protect_purchase_acceptance();
CREATE FUNCTION protect_purchase_acceptance_history() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Acceptance records and history cannot be erased';END $$;
CREATE TRIGGER purchase_acceptance_no_delete BEFORE DELETE ON purchase_acceptances FOR EACH ROW EXECUTE FUNCTION protect_purchase_acceptance_history();
CREATE TRIGGER purchase_acceptance_no_truncate BEFORE TRUNCATE ON purchase_acceptances FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_acceptance_history();
CREATE TRIGGER purchase_acceptance_event_immutable BEFORE UPDATE OR DELETE ON purchase_acceptance_events FOR EACH ROW EXECUTE FUNCTION protect_purchase_acceptance_history();
CREATE TRIGGER purchase_acceptance_event_no_truncate BEFORE TRUNCATE ON purchase_acceptance_events FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_acceptance_history();
CREATE FUNCTION check_purchase_acceptance_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_acceptance_events WHERE subject_id=NEW.id AND after_snapshot=NEW.data AND created_by_id IS NOT DISTINCT FROM NEW.updated_by_id AND created_at=NEW.updated_at AND (TG_OP='INSERT' AND before_snapshot='null'::jsonb OR TG_OP='UPDATE' AND before_snapshot=OLD.data)) THEN RAISE EXCEPTION 'Acceptance mutation requires exact immutable audit';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_acceptance_write_integrity AFTER INSERT OR UPDATE ON purchase_acceptances DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_acceptance_write();
CREATE FUNCTION check_purchase_acceptance_event() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE p uuid;BEGIN
 SELECT project_id INTO p FROM purchase_acceptances WHERE id=NEW.subject_id;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_acceptance' AND entity_type='purchase_acceptance' AND entity_id=NEW.subject_id AND entity_version=(NEW.after_snapshot->>'version')::integer AND action=NEW.action AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted') THEN RAISE EXCEPTION 'Acceptance audit requires central project event';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_acceptance_event_integrity AFTER INSERT ON purchase_acceptance_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_acceptance_event();
COMMIT;
