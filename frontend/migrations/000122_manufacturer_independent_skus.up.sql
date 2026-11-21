BEGIN;

ALTER TABLE public.catalogue_items ALTER COLUMN manufacturer_id DROP NOT NULL;

COMMIT;
