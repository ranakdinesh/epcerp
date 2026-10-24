BEGIN;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM tender.document_drafts) THEN
 RAISE EXCEPTION 'Cannot remove retained document drafts; export/reconcile evidence first'; END IF;
END $$;
DROP TABLE public.document_package_manifest;
DROP TABLE tender.draft_events;
DROP TABLE tender.draft_sources;
ALTER TABLE public.document_package_receipts DROP CONSTRAINT document_package_receipts_check;
ALTER TABLE public.document_package_receipts DROP CONSTRAINT package_single_target;
ALTER TABLE public.document_package_receipts DROP COLUMN draft_id;
ALTER TABLE public.document_package_receipts DROP COLUMN published_at;
ALTER TABLE public.document_package_receipts ADD CONSTRAINT document_package_receipts_check
 CHECK ((state='consumed')=(consumed_by_tender_id IS NOT NULL));
DROP TABLE tender.document_drafts;

CREATE OR REPLACE FUNCTION protect_consumed_package() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF TG_OP='TRUNCATE' THEN RAISE EXCEPTION 'Package evidence requires retention-aware cleanup'; END IF;
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Package evidence cannot be deleted'; END IF;
 IF OLD.state='consumed' THEN RAISE EXCEPTION 'Consumed package is immutable'; END IF;
 IF ROW(NEW.id,NEW.owner_id,NEW.purpose,NEW.limit_profile_revision,NEW.expires_at,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.id,OLD.owner_id,OLD.purpose,OLD.limit_profile_revision,OLD.expires_at,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Package identity is immutable'; END IF;
 RETURN NEW; END $$;
COMMIT;
