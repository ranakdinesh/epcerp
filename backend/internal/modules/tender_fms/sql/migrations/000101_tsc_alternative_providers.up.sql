BEGIN;
ALTER TABLE tender.tsc_workspaces DROP CONSTRAINT tsc_workspaces_mode_check;
ALTER TABLE tender.tsc_workspaces ADD CONSTRAINT tsc_workspaces_mode_check CHECK(mode IN('manual','gemini','openai','deepseek','groq'));
ALTER TABLE tender.tsc_workspace_tasks DROP CONSTRAINT tsc_workspace_tasks_provider_check;
ALTER TABLE tender.tsc_workspace_tasks ADD CONSTRAINT tsc_workspace_tasks_provider_check CHECK(provider IN('gemini','openai','deepseek','groq'));
ALTER TABLE tender.tsc_model_calls DROP CONSTRAINT tsc_model_calls_provider_check;
ALTER TABLE tender.tsc_model_calls ADD CONSTRAINT tsc_model_calls_provider_check CHECK(provider IN('gemini','openai','deepseek','groq'));
ALTER TABLE tender.tsc_model_responses ADD COLUMN input_evidence jsonb NOT NULL DEFAULT '{}'::jsonb;
CREATE TABLE tender.tsc_document_text (
 member_id uuid NOT NULL REFERENCES public.document_package_members(id),reader_version text NOT NULL,
 source_sha256 text NOT NULL CHECK(source_sha256 ~ '^[0-9a-f]{64}$'),
 payload_cipher bytea NOT NULL,payload_hash text NOT NULL CHECK(payload_hash ~ '^[0-9a-f]{64}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),PRIMARY KEY(member_id,reader_version)
);
CREATE TRIGGER tsc_document_text_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON tender.tsc_document_text FOR EACH STATEMENT EXECUTE FUNCTION public.epcerp_reject_event_mutation();
COMMIT;
