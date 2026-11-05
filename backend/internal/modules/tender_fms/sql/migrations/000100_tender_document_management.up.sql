BEGIN;
ALTER TABLE public.document_package_manifest DROP CONSTRAINT document_package_manifest_disposition_check;
ALTER TABLE public.document_package_manifest ADD CONSTRAINT document_package_manifest_disposition_check
 CHECK(disposition IN('listed','verified','directory','blocked','ignored'));
CREATE TABLE tender.draft_document_removals (
 draft_id uuid NOT NULL REFERENCES tender.document_drafts(id),
 member_id uuid NOT NULL REFERENCES public.document_package_members(id),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 created_by_id uuid NOT NULL REFERENCES auth.users(id),
 PRIMARY KEY(draft_id,member_id)
);
CREATE TRIGGER draft_document_removals_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON tender.draft_document_removals
 FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();
COMMIT;
