BEGIN;
CREATE TABLE document_package_receipts (
 id uuid PRIMARY KEY, owner_id uuid NOT NULL REFERENCES users(id), purpose text NOT NULL DEFAULT 'tender_capture' CHECK(purpose='tender_capture'),
 state text NOT NULL DEFAULT 'receiving' CHECK(state IN('receiving','verifying','ready','failed','expired','consumed')),
 version bigint NOT NULL DEFAULT 1 CHECK(version BETWEEN 1 AND 9007199254740991), limit_profile_revision bigint NOT NULL DEFAULT 1 CHECK(limit_profile_revision=1),
 expires_at timestamptz NOT NULL DEFAULT clock_timestamp()+interval '24 hours', consumed_by_tender_id uuid UNIQUE REFERENCES tender_fms_intakes(id),
 claim_id uuid, lease_until timestamptz, attempts integer NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 3), error_code text,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id),
 CHECK((state='consumed')=(consumed_by_tender_id IS NOT NULL)), CHECK(lease_until IS NULL OR claim_id IS NOT NULL)
);
CREATE TABLE document_package_files (
 id uuid PRIMARY KEY, receipt_id uuid NOT NULL REFERENCES document_package_receipts(id), client_file_key uuid NOT NULL,
 filename text NOT NULL CHECK(length(filename) BETWEEN 1 AND 200), kind text NOT NULL CHECK(kind IN('pdf','docx','doc','xlsx','xls','zip')),
 declared_size bigint NOT NULL CHECK(declared_size BETWEEN 1 AND 67108864), declared_sha256 text NOT NULL CHECK(declared_sha256 ~ '^[0-9a-f]{64}$'),
 version bigint NOT NULL DEFAULT 1 CHECK(version BETWEEN 1 AND 9007199254740991), state text NOT NULL DEFAULT 'declared' CHECK(state IN('declared','uploading','received','ready','failed')),
 object_key uuid UNIQUE, upload_key uuid, upload_input_version bigint, claim_id uuid, lease_until timestamptz, error_code text,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id),
 UNIQUE(receipt_id,client_file_key), UNIQUE(receipt_id,id), CHECK(kind='zip' OR declared_size<=33554432),CHECK((claim_id IS NULL)=(lease_until IS NULL)),
 CHECK(state NOT IN('received','ready') OR object_key IS NOT NULL)
);
CREATE TABLE document_package_objects (
 id uuid PRIMARY KEY,receipt_id uuid NOT NULL REFERENCES document_package_receipts(id),file_id uuid NOT NULL REFERENCES document_package_files(id),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),purged_at timestamptz,purge_claim_id uuid,purge_lease_until timestamptz
);
CREATE TABLE document_package_members (
 id uuid PRIMARY KEY, receipt_id uuid NOT NULL REFERENCES document_package_receipts(id), file_id uuid NOT NULL,
 path text NOT NULL CHECK(length(path) BETWEEN 1 AND 2048), normalized_path text NOT NULL,
 filename text NOT NULL CHECK(length(filename) BETWEEN 1 AND 200), kind text NOT NULL CHECK(kind IN('pdf','docx','xlsx')),
 byte_size bigint NOT NULL CHECK(byte_size BETWEEN 1 AND 33554432), expanded_bytes bigint NOT NULL CHECK(expanded_bytes BETWEEN 1 AND 262144000), sha256 text NOT NULL CHECK(sha256 ~ '^[0-9a-f]{64}$'),
 reader_payload bytea NOT NULL, reader_payload_sha256 text NOT NULL CHECK(reader_payload_sha256 ~ '^[0-9a-f]{64}$'),
 validation_claim_id uuid NOT NULL, scanner_revision text NOT NULL, pages integer NOT NULL DEFAULT 0 CHECK(pages BETWEEN 0 AND 1000), sheets integer NOT NULL DEFAULT 0 CHECK(sheets BETWEEN 0 AND 100), cells bigint NOT NULL DEFAULT 0 CHECK(cells BETWEEN 0 AND 100000),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),
 FOREIGN KEY(receipt_id,file_id) REFERENCES document_package_files(receipt_id,id), UNIQUE(receipt_id,validation_claim_id,normalized_path),
 CHECK(octet_length(reader_payload)=byte_size+28)
);
CREATE TABLE document_package_events (
 id uuid PRIMARY KEY,receipt_id uuid NOT NULL REFERENCES document_package_receipts(id),file_id uuid REFERENCES document_package_files(id),
 action text NOT NULL,version bigint NOT NULL CHECK(version>0),error_code text,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id)
);
CREATE TABLE document_package_operations (
 actor_id uuid NOT NULL REFERENCES users(id), command text NOT NULL, context_key text NOT NULL, idempotency_key uuid NOT NULL,
 fingerprint text NOT NULL CHECK(fingerprint ~ '^[0-9a-f]{64}$'), receipt_id uuid NOT NULL REFERENCES document_package_receipts(id),created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(actor_id,command,context_key,idempotency_key)
);
CREATE INDEX document_package_claims ON document_package_receipts(state,lease_until,created_at) WHERE state='verifying';
CREATE TRIGGER document_package_members_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON document_package_members FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
CREATE TRIGGER document_package_events_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON document_package_events FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
CREATE TRIGGER document_package_operations_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON document_package_operations FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
CREATE FUNCTION protect_consumed_package() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF TG_OP='TRUNCATE' THEN RAISE EXCEPTION 'Package evidence requires retention-aware cleanup'; END IF;
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Package evidence cannot be deleted'; END IF;
 IF OLD.state='consumed' THEN RAISE EXCEPTION 'Consumed package is immutable'; END IF;
 IF ROW(NEW.id,NEW.owner_id,NEW.purpose,NEW.limit_profile_revision,NEW.expires_at,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.id,OLD.owner_id,OLD.purpose,OLD.limit_profile_revision,OLD.expires_at,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Package identity is immutable'; END IF;
 RETURN NEW; END $$;
CREATE TRIGGER document_package_identity BEFORE UPDATE OR DELETE ON document_package_receipts FOR EACH ROW EXECUTE FUNCTION protect_consumed_package();
CREATE TRIGGER document_package_no_truncate BEFORE TRUNCATE ON document_package_receipts FOR EACH STATEMENT EXECUTE FUNCTION protect_consumed_package();
CREATE FUNCTION protect_package_file() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Package source cannot be deleted'; END IF;
 IF OLD.state='ready' OR ROW(NEW.id,NEW.receipt_id,NEW.client_file_key,NEW.filename,NEW.kind,NEW.declared_size,NEW.declared_sha256,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.id,OLD.receipt_id,OLD.client_file_key,OLD.filename,OLD.kind,OLD.declared_size,OLD.declared_sha256,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Package source identity is immutable'; END IF;RETURN NEW; END $$;
CREATE TRIGGER document_package_file_identity BEFORE UPDATE OR DELETE ON document_package_files FOR EACH ROW EXECUTE FUNCTION protect_package_file();
CREATE TRIGGER document_package_files_no_truncate BEFORE TRUNCATE ON document_package_files FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
COMMIT;
