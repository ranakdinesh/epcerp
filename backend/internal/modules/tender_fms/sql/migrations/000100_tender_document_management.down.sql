BEGIN;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM tender.draft_document_removals) OR EXISTS(SELECT 1 FROM public.document_package_manifest WHERE disposition='ignored')
 THEN RAISE EXCEPTION 'Cannot discard document management history'; END IF;
END $$;
DROP TABLE tender.draft_document_removals;
ALTER TABLE public.document_package_manifest DROP CONSTRAINT document_package_manifest_disposition_check;
ALTER TABLE public.document_package_manifest ADD CONSTRAINT document_package_manifest_disposition_check CHECK(disposition IN('listed','verified','directory','blocked'));
COMMIT;
