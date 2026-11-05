BEGIN;
-- The new writer owns only cloud/manual TSC work. Historical Go jobs remain intact.
CREATE TABLE tender.tsc_workspaces(
 id uuid PRIMARY KEY,draft_id uuid NOT NULL REFERENCES tender.document_drafts(id),draft_version bigint NOT NULL,
 actor_id uuid NOT NULL REFERENCES auth.users(id),operation_key uuid NOT NULL,request_hash text NOT NULL,
 mode text NOT NULL CHECK(mode IN('manual','gemini','openai')),state text NOT NULL CHECK(state IN('queued','processing','ready','failed')),
 version bigint NOT NULL DEFAULT 1 CHECK(version>0),snapshot_cipher bytea NOT NULL,snapshot_hash text NOT NULL,
 answers_cipher bytea NOT NULL,answers_hash text NOT NULL,claim_id uuid,lease_until timestamptz,error_code text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(draft_id,actor_id,operation_key),UNIQUE(id,draft_id),CHECK((state='processing')=(claim_id IS NOT NULL AND lease_until IS NOT NULL))
);
CREATE UNIQUE INDEX tsc_workspace_pending ON tender.tsc_workspaces(draft_id) WHERE state IN('queued','processing');
CREATE TABLE tender.tsc_workspace_tasks(
 id uuid PRIMARY KEY,job_id uuid NOT NULL,draft_id uuid NOT NULL,actor_id uuid NOT NULL REFERENCES auth.users(id),
 operation_key uuid NOT NULL,request_hash text NOT NULL,question_id text,provider text NOT NULL CHECK(provider IN('gemini','openai')),
 stronger boolean NOT NULL DEFAULT false,state text NOT NULL DEFAULT 'queued' CHECK(state IN('queued','processing','ready','failed')),
 claim_id uuid,lease_until timestamptz,error_code text NOT NULL DEFAULT '',created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 FOREIGN KEY(job_id,draft_id) REFERENCES tender.tsc_workspaces(id,draft_id),UNIQUE(job_id,operation_key),
 CHECK((state='processing')=(claim_id IS NOT NULL AND lease_until IS NOT NULL))
);
CREATE UNIQUE INDEX tsc_task_pending ON tender.tsc_workspace_tasks(job_id) WHERE state IN('queued','processing');
CREATE TABLE tender.tsc_model_calls(
 id uuid PRIMARY KEY,job_id uuid NOT NULL,draft_id uuid NOT NULL,actor_id uuid NOT NULL REFERENCES auth.users(id),
 task_id uuid NOT NULL REFERENCES tender.tsc_workspace_tasks(id),request_key text NOT NULL,provider text NOT NULL CHECK(provider IN('gemini','openai')),model text NOT NULL,purpose text NOT NULL,
 manifest jsonb NOT NULL,pricing jsonb NOT NULL,reserved_paise bigint NOT NULL CHECK(reserved_paise BETWEEN 1 AND 10000),
 state text NOT NULL DEFAULT 'reserved' CHECK(state IN('reserved','sent','completed','failed','uncertain')),
 error_code text NOT NULL DEFAULT '',created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 FOREIGN KEY(job_id,draft_id) REFERENCES tender.tsc_workspaces(id,draft_id),UNIQUE(job_id,request_key),UNIQUE(id,job_id,draft_id)
);
CREATE TABLE tender.tsc_model_responses(
 id uuid PRIMARY KEY,call_id uuid NOT NULL UNIQUE,job_id uuid NOT NULL,draft_id uuid NOT NULL,
 http_status integer NOT NULL,provider_request_id text NOT NULL DEFAULT '',raw_cipher bytea NOT NULL,raw_hash text NOT NULL,
 parsed_cipher bytea NOT NULL,parsed_hash text NOT NULL,usage jsonb NOT NULL,estimated_micro_inr bigint CHECK(estimated_micro_inr>=0),
 cost_basis text NOT NULL CHECK(cost_basis IN('reported_usage','free_tier','usage_unavailable')),created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 FOREIGN KEY(call_id,job_id,draft_id) REFERENCES tender.tsc_model_calls(id,job_id,draft_id)
);
CREATE TABLE tender.tsc_workspace_events(
 id uuid PRIMARY KEY,job_id uuid NOT NULL REFERENCES tender.tsc_workspaces(id),version bigint NOT NULL,action text NOT NULL,
 reason text NOT NULL DEFAULT '',payload_cipher bytea NOT NULL,payload_hash text NOT NULL,created_by_id uuid NOT NULL REFERENCES auth.users(id),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),UNIQUE(job_id,version)
);
CREATE TRIGGER tsc_model_responses_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON tender.tsc_model_responses FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();
CREATE TRIGGER tsc_workspace_events_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON tender.tsc_workspace_events FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();
CREATE FUNCTION tender.protect_tsc_workspace() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF TG_OP<>'UPDATE' THEN RAISE EXCEPTION 'TSC workspace requires retention-aware cleanup';END IF;
 IF ROW(NEW.id,NEW.draft_id,NEW.draft_version,NEW.actor_id,NEW.operation_key,NEW.request_hash,NEW.mode,NEW.snapshot_cipher,NEW.snapshot_hash,NEW.created_at)
 IS DISTINCT FROM ROW(OLD.id,OLD.draft_id,OLD.draft_version,OLD.actor_id,OLD.operation_key,OLD.request_hash,OLD.mode,OLD.snapshot_cipher,OLD.snapshot_hash,OLD.created_at)
 THEN RAISE EXCEPTION 'TSC source snapshot is immutable';END IF;
 RETURN NEW;END $$;
CREATE TRIGGER tsc_workspace_immutable BEFORE UPDATE OR DELETE ON tender.tsc_workspaces FOR EACH ROW EXECUTE FUNCTION tender.protect_tsc_workspace();
CREATE TRIGGER tsc_workspace_no_truncate BEFORE TRUNCATE ON tender.tsc_workspaces FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();
CREATE FUNCTION tender.protect_tsc_model_call() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF TG_OP<>'UPDATE' OR OLD.state IN('completed','failed','uncertain') THEN RAISE EXCEPTION 'Model call evidence is immutable';END IF;
 IF ROW(NEW.id,NEW.job_id,NEW.draft_id,NEW.actor_id,NEW.task_id,NEW.request_key,NEW.provider,NEW.model,NEW.purpose,NEW.manifest,NEW.pricing,NEW.reserved_paise,NEW.created_at)
 IS DISTINCT FROM ROW(OLD.id,OLD.job_id,OLD.draft_id,OLD.actor_id,OLD.task_id,OLD.request_key,OLD.provider,OLD.model,OLD.purpose,OLD.manifest,OLD.pricing,OLD.reserved_paise,OLD.created_at)
 THEN RAISE EXCEPTION 'Model request identity is immutable';END IF;
 RETURN NEW;END $$;
CREATE TRIGGER tsc_model_call_immutable BEFORE UPDATE OR DELETE ON tender.tsc_model_calls FOR EACH ROW EXECUTE FUNCTION tender.protect_tsc_model_call();
CREATE TRIGGER tsc_model_call_no_truncate BEFORE TRUNCATE ON tender.tsc_model_calls FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();
CREATE FUNCTION tender.protect_tsc_workspace_task() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF TG_OP<>'UPDATE' OR OLD.state IN('ready','failed') THEN RAISE EXCEPTION 'TSC task evidence is immutable';END IF;
 IF ROW(NEW.id,NEW.job_id,NEW.draft_id,NEW.actor_id,NEW.operation_key,NEW.request_hash,NEW.question_id,NEW.provider,NEW.stronger,NEW.created_at)
 IS DISTINCT FROM ROW(OLD.id,OLD.job_id,OLD.draft_id,OLD.actor_id,OLD.operation_key,OLD.request_hash,OLD.question_id,OLD.provider,OLD.stronger,OLD.created_at)
 THEN RAISE EXCEPTION 'TSC task identity is immutable';END IF;RETURN NEW;END $$;
CREATE TRIGGER tsc_task_immutable BEFORE UPDATE OR DELETE ON tender.tsc_workspace_tasks FOR EACH ROW EXECUTE FUNCTION tender.protect_tsc_workspace_task();
CREATE TRIGGER tsc_task_no_truncate BEFORE TRUNCATE ON tender.tsc_workspace_tasks FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();
COMMIT;
