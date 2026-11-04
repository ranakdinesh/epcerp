BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM tender.tsc_workspaces) THEN RAISE EXCEPTION 'Retained TSC responses and edits prevent destructive rollback';END IF;END $$;
DROP TABLE tender.tsc_workspace_events,tender.tsc_model_responses,tender.tsc_model_calls,tender.tsc_workspace_tasks,tender.tsc_workspaces;
DROP FUNCTION tender.protect_tsc_workspace_task();DROP FUNCTION tender.protect_tsc_model_call();DROP FUNCTION tender.protect_tsc_workspace();
COMMIT;
