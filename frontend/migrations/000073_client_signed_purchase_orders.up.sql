BEGIN;
ALTER TABLE purchase_order_events DROP CONSTRAINT purchase_order_events_action_check;
ALTER TABLE purchase_order_events ADD CONSTRAINT purchase_order_events_action_check CHECK(action IN('prepare','release','submit_client','client_decision'));
CREATE TABLE purchase_order_client_files (
 id uuid PRIMARY KEY, order_id uuid NOT NULL REFERENCES purchase_orders(id),draft_revision integer NOT NULL CHECK(draft_revision>0),
 name varchar(255) NOT NULL,media_type varchar(80) NOT NULL CHECK(media_type IN('application/pdf','image/png','image/jpeg')),
 byte_size bigint NOT NULL CHECK(byte_size BETWEEN 1 AND 10485760),sha256 char(64) NOT NULL,
 object_bucket text NOT NULL,object_key text NOT NULL UNIQUE,
 is_active boolean NOT NULL DEFAULT true,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id)
);
CREATE TABLE purchase_order_client_file_events (
 id uuid PRIMARY KEY,file_id uuid NOT NULL REFERENCES purchase_order_client_files(id),order_id uuid NOT NULL REFERENCES purchase_orders(id),action varchar(20) NOT NULL CHECK(action IN('uploaded','downloaded')),
 created_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id)
);
CREATE TRIGGER po_client_file_immutable BEFORE UPDATE OR DELETE ON purchase_order_client_files FOR EACH ROW EXECUTE FUNCTION protect_purchase_order_history();
CREATE TRIGGER po_client_file_no_truncate BEFORE TRUNCATE ON purchase_order_client_files FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_order_history();
CREATE TRIGGER po_client_file_event_immutable BEFORE UPDATE OR DELETE ON purchase_order_client_file_events FOR EACH ROW EXECUTE FUNCTION protect_purchase_order_history();
CREATE TRIGGER po_client_file_event_no_truncate BEFORE TRUNCATE ON purchase_order_client_file_events FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_order_history();
CREATE FUNCTION check_po_client_signoff() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE a jsonb;
BEGIN
 a=NEW.data->'client_approval';
 IF NEW.state='released' AND OLD.state<>'released' THEN
  IF coalesce(a->>'status','')<>'approved' OR coalesce((NEW.data->>'draft_revision')::integer,0)<1 OR a->>'draft_revision' IS DISTINCT FROM NEW.data->>'draft_revision' OR a->'decision'->>'status' IS DISTINCT FROM 'approved' OR coalesce(a->>'recorded_by_id','')='' OR a->>'recorded_at' IS NULL THEN RAISE EXCEPTION 'Client sign-off for this draft is required before PO release'; END IF;
  IF NOT EXISTS(SELECT 1 FROM purchase_order_client_files f WHERE f.id::text=a->'decision'->>'file_id' AND f.order_id=NEW.id AND f.draft_revision=(NEW.data->>'draft_revision')::integer AND f.is_active) THEN RAISE EXCEPTION 'Actual signed PO file must match this order revision';END IF;
  IF NEW.data->'client_approval' IS DISTINCT FROM OLD.data->'client_approval' THEN RAISE EXCEPTION 'Release must retain previously recorded client approval';END IF;
 END IF;
 IF NEW.data->'details' IS DISTINCT FROM OLD.data->'details' OR NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot THEN
  IF coalesce(a->>'status','')='approved' THEN RAISE EXCEPTION 'Edited PO must obtain new client approval';END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER po_client_signoff_guard BEFORE UPDATE ON purchase_orders FOR EACH ROW EXECUTE FUNCTION check_po_client_signoff();
CREATE FUNCTION check_po_client_file_event() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE p uuid;
BEGIN
 SELECT o.project_id INTO p FROM purchase_orders o JOIN purchase_order_client_files f ON f.order_id=o.id WHERE f.id=NEW.file_id AND o.id=NEW.order_id;
 IF p IS NULL OR NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_orders' AND entity_type='purchase_order_client_file' AND entity_id=NEW.file_id AND action=NEW.action AND actor_id=NEW.created_by_id AND scope_id=p AND visibility='restricted') THEN RAISE EXCEPTION 'Signed PO evidence requires scoped immutable audit';END IF;
 RETURN NEW;
END $$;
CREATE CONSTRAINT TRIGGER po_client_file_event_integrity AFTER INSERT ON purchase_order_client_file_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_po_client_file_event();
CREATE FUNCTION check_po_client_file_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_orders o WHERE o.id=NEW.order_id AND o.state='draft' AND o.data->'client_approval'->>'status'='submitted' AND (o.data->>'draft_revision')::integer=NEW.draft_revision) THEN RAISE EXCEPTION 'Signed file requires matching submitted PO revision';END IF;
 IF NOT EXISTS(SELECT 1 FROM purchase_order_client_file_events WHERE file_id=NEW.id AND order_id=NEW.order_id AND action='uploaded' AND created_by_id=NEW.created_by_id AND created_at=NEW.created_at) THEN RAISE EXCEPTION 'Signed file upload requires audit';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER po_client_file_integrity AFTER INSERT ON purchase_order_client_files DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_po_client_file_write();
COMMIT;
