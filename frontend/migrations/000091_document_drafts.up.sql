BEGIN;
CREATE TABLE tender.document_drafts (
 id uuid PRIMARY KEY, name text NOT NULL CHECK(length(name) BETWEEN 2 AND 255),
 owner_id uuid NOT NULL REFERENCES auth.users(id), version bigint NOT NULL DEFAULT 1,
 is_active boolean NOT NULL DEFAULT true, operation_key uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(), updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 created_by_id uuid REFERENCES auth.users(id), updated_by_id uuid REFERENCES auth.users(id),
 UNIQUE(owner_id,operation_key)
);
ALTER TABLE public.document_package_receipts ADD COLUMN draft_id uuid REFERENCES tender.document_drafts(id);
ALTER TABLE public.document_package_receipts ADD COLUMN published_at timestamptz;
ALTER TABLE public.document_package_receipts DROP CONSTRAINT document_package_receipts_check;
ALTER TABLE public.document_package_receipts ADD CONSTRAINT document_package_receipts_check
 CHECK ((state='consumed') = (consumed_by_tender_id IS NOT NULL OR draft_id IS NOT NULL AND published_at IS NOT NULL));

CREATE TABLE tender.draft_sources (
 draft_id uuid NOT NULL REFERENCES tender.document_drafts(id),
 member_id uuid PRIMARY KEY REFERENCES public.document_package_members(id),
 document_id uuid NOT NULL REFERENCES public.document_readings(id),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES auth.users(id)
);
CREATE TABLE tender.draft_events (
 id uuid PRIMARY KEY, draft_id uuid NOT NULL REFERENCES tender.document_drafts(id),
 action text NOT NULL, receipt_id uuid REFERENCES public.document_package_receipts(id),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(), created_by_id uuid REFERENCES auth.users(id)
);
ALTER TABLE public.document_package_receipts ADD CONSTRAINT package_single_target CHECK (NOT(consumed_by_tender_id IS NOT NULL AND draft_id IS NOT NULL));
CREATE INDEX document_draft_owner ON tender.document_drafts(owner_id,created_at DESC);
CREATE INDEX package_draft_ready ON public.document_package_receipts(draft_id,state) WHERE draft_id IS NOT NULL;

CREATE OR REPLACE FUNCTION public.protect_consumed_package() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF TG_OP='TRUNCATE' THEN RAISE EXCEPTION 'Package evidence requires retention-aware cleanup'; END IF;
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Package evidence cannot be deleted'; END IF;
 IF OLD.state='consumed' THEN RAISE EXCEPTION 'Consumed package is immutable'; END IF;
 IF ROW(NEW.id,NEW.owner_id,NEW.purpose,NEW.limit_profile_revision,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.id,OLD.owner_id,OLD.purpose,OLD.limit_profile_revision,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Package identity is immutable'; END IF;
 IF OLD.draft_id IS NOT NULL AND NEW.draft_id IS DISTINCT FROM OLD.draft_id THEN RAISE EXCEPTION 'Retained draft is immutable'; END IF;
 IF NEW.expires_at IS DISTINCT FROM OLD.expires_at AND NOT(OLD.draft_id IS NULL AND NEW.draft_id IS NOT NULL AND NEW.expires_at='9999-12-31'::timestamptz) THEN RAISE EXCEPTION 'Package expiry is immutable'; END IF;
 RETURN NEW; END $$;
CREATE TRIGGER draft_sources_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON tender.draft_sources FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();
CREATE TRIGGER draft_events_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON tender.draft_events FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();
CREATE TABLE public.document_package_manifest (
 id uuid PRIMARY KEY,receipt_id uuid NOT NULL REFERENCES public.document_package_receipts(id),
 file_id uuid NOT NULL REFERENCES public.document_package_files(id),claim_id uuid NOT NULL,
 path text NOT NULL, disposition text NOT NULL CHECK(disposition IN('listed','verified','directory','blocked')),
 error_code text NOT NULL DEFAULT '',created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX package_manifest_claim ON public.document_package_manifest(receipt_id,claim_id,created_at);
CREATE TRIGGER package_manifest_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON public.document_package_manifest FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();
COMMIT;
