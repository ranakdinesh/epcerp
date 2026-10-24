BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM quotation_approval_sets q CROSS JOIN LATERAL jsonb_array_elements(q.attempts) a CROSS JOIN LATERAL jsonb_array_elements(a->'lines') l WHERE a->>'state' IN('approved','rejected') AND coalesce(length(btrim(l->>'comment')),0)=0) THEN RAISE EXCEPTION 'Retain decisions with optional item notes';END IF;END $$;
CREATE OR REPLACE FUNCTION check_quotation_approval_decisions() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE a jsonb;d finance_quotation_decisions;BEGIN
 FOR a IN SELECT value FROM jsonb_array_elements(NEW.attempts) LOOP
  IF a->>'state' IN('approved','rejected') THEN
   IF EXISTS(SELECT 1 FROM jsonb_array_elements(a->'lines') l WHERE l->>'decision' NOT IN('accept','reject') OR l->>'decision' IS NULL OR coalesce(length(btrim(l->>'comment')),0)=0) OR (a->>'state'='rejected') IS DISTINCT FROM EXISTS(SELECT 1 FROM jsonb_array_elements(a->'lines') l WHERE l->>'decision'='reject') THEN RAISE EXCEPTION 'Every item decision must match the Finance outcome';END IF;
   SELECT * INTO d FROM finance_quotation_decisions WHERE id=(a->>'decision_id')::uuid AND attempt_id=(a->>'id')::uuid AND bo_id=NEW.bo_id AND project_id=NEW.project_id;
   IF NOT FOUND OR d.outcome IS DISTINCT FROM a->>'state' OR d.currency IS DISTINCT FROM a->>'currency' OR d.amount IS DISTINCT FROM (a->>'total')::numeric OR d.submitted_by_id::text IS DISTINCT FROM a->>'submitted_by_id' OR d.created_by_id::text IS DISTINCT FROM a->>'reviewed_by_id' OR d.source_version>=NEW.version OR a->>'reviewed_at' IS NULL THEN RAISE EXCEPTION 'Quotation outcome requires its matching Finance decision';END IF;
  ELSIF a->>'state' IS DISTINCT FROM 'pending_accounts' OR a->>'decision_id' IS NOT NULL OR a->>'reviewed_by_id' IS NOT NULL OR a->>'reviewed_at' IS NOT NULL THEN RAISE EXCEPTION 'Quotation request state is invalid';END IF;
 END LOOP;
 RETURN NEW;END $$;
COMMIT;
