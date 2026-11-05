-- Retired import columns remain as historical provenance, outside normal client entry.
ALTER TABLE clients ALTER COLUMN legacy_source SET DEFAULT '',
 ALTER COLUMN legacy_source_id SET DEFAULT '';
