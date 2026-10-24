BEGIN;
CREATE TABLE purchase_loi_reviews (
 id uuid PRIMARY KEY,
 document_id uuid NOT NULL UNIQUE REFERENCES purchase_loi_documents(id),
 details_correct boolean NOT NULL CHECK(details_correct),
 prices_correct boolean NOT NULL CHECK(prices_correct),
 format_correct boolean NOT NULL CHECK(format_correct),
 is_active boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL,
 created_by_id uuid NOT NULL REFERENCES users(id), updated_by_id uuid NOT NULL REFERENCES users(id)
);
CREATE FUNCTION check_purchase_loi_review_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM business_events e WHERE e.id=NEW.id AND e.source_module='purchase_awards' AND e.entity_type='loi_document_review' AND e.entity_id=NEW.id AND e.entity_version=1 AND e.action='completed' AND e.actor_id=NEW.created_by_id AND e.actor_kind='user' AND e.scope_kind='project' AND e.visibility='restricted' AND e.scope_id=(SELECT project_id FROM purchase_loi_documents WHERE id=NEW.document_id)) THEN RAISE EXCEPTION 'LOI review requires immutable project audit'; END IF;
 RETURN NEW; END $$;
CREATE CONSTRAINT TRIGGER purchase_loi_review_audit AFTER INSERT ON purchase_loi_reviews DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_loi_review_audit();
CREATE TRIGGER purchase_loi_review_immutable BEFORE UPDATE OR DELETE ON purchase_loi_reviews FOR EACH ROW EXECUTE FUNCTION protect_purchase_sourcing_history();
CREATE TRIGGER purchase_loi_review_no_truncate BEFORE TRUNCATE ON purchase_loi_reviews FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_sourcing_history();
COMMIT;
