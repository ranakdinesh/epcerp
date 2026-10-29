BEGIN;
CREATE TABLE tender.tsc_jobs (
 id uuid PRIMARY KEY, draft_id uuid NOT NULL REFERENCES tender.document_drafts(id),
 draft_version bigint NOT NULL CHECK(draft_version>0), actor_id uuid NOT NULL REFERENCES auth.users(id),
 operation_key uuid NOT NULL, mode text NOT NULL CHECK(mode IN ('manual','ollama')), model text NOT NULL,
 state text NOT NULL DEFAULT 'queued' CHECK(state IN('queued','processing','ready','failed')),
 version bigint NOT NULL DEFAULT 1, attempts integer NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 3),
 claim_id uuid, lease_until timestamptz, error_code text NOT NULL DEFAULT '',
 input_cipher bytea NOT NULL, input_hash text NOT NULL, result_cipher bytea, result_hash text,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(), updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(draft_id,actor_id,operation_key),
 CHECK((state='processing')=(claim_id IS NOT NULL AND lease_until IS NOT NULL)),
 CHECK((result_cipher IS NULL)=(result_hash IS NULL))
);
CREATE UNIQUE INDEX tsc_one_pending ON tender.tsc_jobs(draft_id) WHERE state IN('queued','processing');
CREATE INDEX tsc_claim_queue ON tender.tsc_jobs(state,created_at);
CREATE FUNCTION tender.protect_tsc_inputs() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF TG_OP<>'UPDATE' THEN RAISE EXCEPTION 'TSC snapshots require retention-aware cleanup'; END IF;
 IF ROW(NEW.id,NEW.draft_id,NEW.draft_version,NEW.actor_id,NEW.operation_key,NEW.mode,NEW.model,NEW.input_cipher,NEW.input_hash,NEW.created_at)
 IS DISTINCT FROM ROW(OLD.id,OLD.draft_id,OLD.draft_version,OLD.actor_id,OLD.operation_key,OLD.mode,OLD.model,OLD.input_cipher,OLD.input_hash,OLD.created_at)
 THEN RAISE EXCEPTION 'TSC input snapshot is immutable'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER tsc_inputs_immutable BEFORE UPDATE OR DELETE ON tender.tsc_jobs FOR EACH ROW EXECUTE FUNCTION tender.protect_tsc_inputs();
CREATE TRIGGER tsc_jobs_no_truncate BEFORE TRUNCATE ON tender.tsc_jobs FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();
CREATE TABLE tender.tsc_events (
 id uuid PRIMARY KEY, job_id uuid NOT NULL REFERENCES tender.tsc_jobs(id),
 action text NOT NULL, version bigint NOT NULL, reason text NOT NULL DEFAULT '',
 result_cipher bytea, result_hash text, created_by_id uuid REFERENCES auth.users(id),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TRIGGER tsc_events_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON tender.tsc_events FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();
COMMIT;
