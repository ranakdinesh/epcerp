BEGIN;
CREATE TABLE purchase_loi_documents (
 id uuid PRIMARY KEY, project_id uuid NOT NULL REFERENCES projects(id), handoff_id uuid NOT NULL REFERENCES purchase_award_handoffs(id), attempt_id uuid NOT NULL,
 version integer NOT NULL CHECK(version>0), reference text NOT NULL CHECK(length(trim(reference)) BETWEEN 1 AND 120), document_date date NOT NULL, format text NOT NULL CHECK(format IN('general','cable','panel','pole')),
 request_hash text NOT NULL CHECK(length(request_hash)=64), content jsonb NOT NULL CHECK(jsonb_typeof(content)='object'), sha256 text NOT NULL CHECK(length(sha256)=64),
 object_bucket text NOT NULL,object_key text NOT NULL UNIQUE,byte_size bigint NOT NULL CHECK(byte_size>0),
 is_active boolean NOT NULL DEFAULT true,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id),
 UNIQUE(handoff_id,version),UNIQUE(handoff_id,request_hash)
);
CREATE INDEX purchase_loi_project_attempt ON purchase_loi_documents(project_id,attempt_id,version DESC);
CREATE TABLE purchase_loi_document_events(id uuid PRIMARY KEY,document_id uuid NOT NULL REFERENCES purchase_loi_documents(id),action text NOT NULL CHECK(action IN('generated','downloaded')),created_at timestamptz NOT NULL,created_by_id uuid NOT NULL REFERENCES users(id));
CREATE FUNCTION check_purchase_loi_handoff() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_award_handoffs h WHERE h.id=NEW.handoff_id AND h.project_id=NEW.project_id AND h.attempt_id=NEW.attempt_id AND h.is_active) THEN RAISE EXCEPTION 'LOI must belong to its approved award handoff';END IF;
 RETURN NEW;END $$;
CREATE TRIGGER purchase_loi_handoff BEFORE INSERT ON purchase_loi_documents FOR EACH ROW EXECUTE FUNCTION check_purchase_loi_handoff();
CREATE FUNCTION check_purchase_loi_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM business_events e WHERE e.id=NEW.id AND e.source_module='purchase_awards' AND e.entity_type='vendor_loi_document_event' AND e.entity_id=NEW.id AND e.entity_version=1 AND e.action=NEW.action AND e.actor_id=NEW.created_by_id AND e.actor_kind='user' AND e.scope_kind='project' AND e.visibility='restricted' AND e.scope_id=(SELECT project_id FROM purchase_loi_documents WHERE id=NEW.document_id)) THEN RAISE EXCEPTION 'LOI access requires immutable project audit';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_loi_audit AFTER INSERT ON purchase_loi_document_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_loi_audit();
CREATE FUNCTION check_purchase_loi_generation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_loi_document_events e WHERE e.document_id=NEW.id AND e.action='generated' AND e.created_by_id=NEW.created_by_id AND e.created_at=NEW.created_at) THEN RAISE EXCEPTION 'LOI generation requires its audit';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_loi_generation AFTER INSERT ON purchase_loi_documents DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_loi_generation();
CREATE TRIGGER purchase_loi_immutable BEFORE UPDATE OR DELETE ON purchase_loi_documents FOR EACH ROW EXECUTE FUNCTION protect_purchase_sourcing_history();
CREATE TRIGGER purchase_loi_no_truncate BEFORE TRUNCATE ON purchase_loi_documents FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_sourcing_history();
CREATE TRIGGER purchase_loi_events_immutable BEFORE UPDATE OR DELETE ON purchase_loi_document_events FOR EACH ROW EXECUTE FUNCTION protect_purchase_sourcing_history();
CREATE TRIGGER purchase_loi_events_no_truncate BEFORE TRUNCATE ON purchase_loi_document_events FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_sourcing_history();
COMMIT;
