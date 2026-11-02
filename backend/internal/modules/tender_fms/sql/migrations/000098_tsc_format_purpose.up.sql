BEGIN;
ALTER TABLE tender.tsc_formats ADD COLUMN purpose text NOT NULL DEFAULT 'Generic' CHECK (char_length(btrim(purpose)) BETWEEN 1 AND 80);
COMMIT;
