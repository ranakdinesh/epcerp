BEGIN;
ALTER TABLE tender.tsc_jobs DROP CONSTRAINT tsc_jobs_mode_check;
ALTER TABLE tender.tsc_jobs ADD CONSTRAINT tsc_jobs_mode_check CHECK(mode IN ('manual','ollama','gemini','openai'));
CREATE TABLE tender.tsc_cloud_calls (
 id uuid PRIMARY KEY, draft_id uuid NOT NULL REFERENCES tender.document_drafts(id),
 job_id uuid NOT NULL REFERENCES tender.tsc_jobs(id), actor_id uuid NOT NULL REFERENCES auth.users(id),
 request_key text NOT NULL CHECK(length(request_key) BETWEEN 1 AND 200),
 fingerprint text NOT NULL, provider text NOT NULL CHECK(provider IN ('gemini','openai')),
 model text NOT NULL, reserved_paise bigint NOT NULL CHECK(reserved_paise BETWEEN 1 AND 10000),
 state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','ready','failed')),
 error_code text NOT NULL DEFAULT '', result_cipher bytea, result_hash text,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(), updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(draft_id,request_key), CHECK((result_cipher IS NULL)=(result_hash IS NULL)),
 CHECK((state='ready')=(result_cipher IS NOT NULL))
);
CREATE INDEX tsc_cloud_draft_budget ON tender.tsc_cloud_calls(draft_id);
CREATE FUNCTION tender.protect_cloud_reservation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF TG_OP<>'UPDATE' THEN RAISE EXCEPTION 'Cloud reservations cannot be deleted'; END IF;
 IF OLD.state<>'pending' OR ROW(NEW.id,NEW.draft_id,NEW.job_id,NEW.actor_id,NEW.request_key,NEW.fingerprint,NEW.provider,NEW.model,NEW.reserved_paise,NEW.created_at)
 IS DISTINCT FROM ROW(OLD.id,OLD.draft_id,OLD.job_id,OLD.actor_id,OLD.request_key,OLD.fingerprint,OLD.provider,OLD.model,OLD.reserved_paise,OLD.created_at)
 THEN RAISE EXCEPTION 'Cloud reservation evidence is immutable'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER cloud_reservation_immutable BEFORE UPDATE OR DELETE ON tender.tsc_cloud_calls FOR EACH ROW EXECUTE FUNCTION tender.protect_cloud_reservation();
CREATE TRIGGER cloud_reservation_no_truncate BEFORE TRUNCATE ON tender.tsc_cloud_calls FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();
COMMIT;
