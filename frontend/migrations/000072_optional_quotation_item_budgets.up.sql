BEGIN;
-- Bind reviewed budget values only at acceptance; commercial proposal snapshots stay frozen.
CREATE OR REPLACE FUNCTION protect_quotation_approval_scope() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE a jsonb;n jsonb;l jsonb;nl jsonb;src jsonb;k text;review_fields text[];line_fields text[];baseline jsonb;allocation jsonb;bind_budget boolean;BEGIN
 IF TG_OP='UPDATE' AND (NEW.bo_id IS DISTINCT FROM OLD.bo_id OR NEW.project_id IS DISTINCT FROM OLD.project_id OR NEW.created_at IS DISTINCT FROM OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1) THEN RAISE EXCEPTION 'Quotation approval ownership/version is immutable';END IF;
 IF TG_OP='UPDATE' THEN
  FOR a IN SELECT value FROM jsonb_array_elements(OLD.attempts) LOOP
   SELECT value INTO n FROM jsonb_array_elements(NEW.attempts) WHERE value->>'id'=a->>'id';
   IF NOT FOUND THEN RAISE EXCEPTION 'Quotation submissions must be retained';END IF;
   IF a->>'state'<>'pending_accounts' THEN
    IF n IS DISTINCT FROM a THEN RAISE EXCEPTION 'Decided quotations are immutable';END IF;
    CONTINUE;
   END IF;
   review_fields:=ARRAY['state','lines','reviewed_by_id','reviewed_at','decision_id','return_action','review_comment'];
   line_fields:=ARRAY['decision','comment'];
   bind_budget:=a->>'state'='pending_accounts' AND n->>'state'='approved';
   IF bind_budget THEN
    review_fields:=review_fields||ARRAY['budget_revision','budget_issue'];
    line_fields:=line_fields||ARRAY['budget','difference'];
    SELECT approved_snapshot INTO baseline FROM project_budgets WHERE project_id=NEW.project_id AND is_active;
    IF baseline IS NULL OR baseline='null'::jsonb OR coalesce((n->>'budget_revision')::integer,0)<1 OR n->>'budget_revision' IS DISTINCT FROM baseline->>'revision' OR n->>'currency' IS DISTINCT FROM baseline->'data'->>'currency' OR coalesce(n->>'budget_issue','')<>'' THEN RAISE EXCEPTION 'Vendor approval requires the current approved project budget';END IF;
   END IF;
   IF jsonb_array_length(coalesce(a->'proposals','[]'))=0 THEN
    IF n-review_fields IS DISTINCT FROM a-review_fields THEN RAISE EXCEPTION 'Submitted quotation source is immutable';END IF;
    src:=a;
   ELSE
    IF n-(review_fields||ARRAY['quote_key','document_id','revision','vendor_id','vendor','reference','total','verification_fingerprint','terms','valid_until']) IS DISTINCT FROM a-(review_fields||ARRAY['quote_key','document_id','revision','vendor_id','vendor','reference','total','verification_fingerprint','terms','valid_until']) THEN RAISE EXCEPTION 'Submitted shortlist and executive reasons are immutable';END IF;
    SELECT value->'snapshot' INTO src FROM jsonb_array_elements(a->'proposals') WHERE value->>'quote_key'=n->>'quote_key';
    IF NOT FOUND THEN RAISE EXCEPTION 'Finance must choose a submitted proposal';END IF;
    FOREACH k IN ARRAY ARRAY['quote_key','document_id','revision','vendor_id','vendor','reference','total','verification_fingerprint','terms','valid_until'] LOOP
     IF n->k IS DISTINCT FROM src->k THEN RAISE EXCEPTION 'Selected proposal source must match the frozen snapshot';END IF;
    END LOOP;
    IF n->>'state'='pending_accounts' AND n IS DISTINCT FROM a THEN RAISE EXCEPTION 'Waiting shortlist is frozen';END IF;
   END IF;
   IF n->>'state'='rejected' AND jsonb_array_length(coalesce(a->'proposals','[]'))>0 AND coalesce(n->>'return_action','') NOT IN('negotiate','change_vendor') THEN RAISE EXCEPTION 'Rejected shortlist requires recovery instruction';END IF;
   IF n->>'state'='approved' AND coalesce(n->>'return_action','')<>'' THEN RAISE EXCEPTION 'Approved shortlist cannot be returned';END IF;
   IF jsonb_array_length(n->'lines')<>jsonb_array_length(src->'lines') THEN RAISE EXCEPTION 'Every submitted item must be retained';END IF;
   FOR l IN SELECT value FROM jsonb_array_elements(src->'lines') LOOP
    SELECT value INTO nl FROM jsonb_array_elements(n->'lines') WHERE value->>'line_id'=l->>'line_id';
    IF NOT FOUND OR nl-line_fields IS DISTINCT FROM l-line_fields THEN RAISE EXCEPTION 'Submitted item source is immutable';END IF;
    IF bind_budget THEN
     SELECT value INTO allocation FROM jsonb_array_elements(baseline->'data'->'allocations') WHERE value->>'bo_id'=NEW.bo_id::text AND value->>'line_id'=nl->>'line_id' AND coalesce((value->>'is_active')::boolean,false);
     IF NOT FOUND THEN
      IF nl->>'budget' IS NOT NULL OR nl->>'difference' IS NOT NULL THEN RAISE EXCEPTION 'Unallocated items must retain unavailable comparisons';END IF;
     ELSIF nl->>'budget' IS NULL OR nl->>'difference' IS NULL OR (nl->>'budget')::numeric IS DISTINCT FROM (allocation->>'amount')::numeric OR (nl->>'difference')::numeric IS DISTINCT FROM (nl->>'quoted')::numeric-(allocation->>'amount')::numeric THEN RAISE EXCEPTION 'Finance item comparison must match its approved allocation';END IF;
    END IF;
   END LOOP;
  END LOOP;
 END IF;
 IF NOT EXISTS(SELECT 1 FROM purchase_bo_lists WHERE id=NEW.bo_id AND project_id=NEW.project_id) THEN RAISE EXCEPTION 'Quotation approval BO belongs to another project';END IF;
 RETURN NEW;END $$;
COMMIT;
