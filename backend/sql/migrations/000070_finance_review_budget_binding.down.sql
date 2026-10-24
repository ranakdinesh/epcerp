BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM quotation_approval_sets q CROSS JOIN LATERAL jsonb_array_elements(q.attempts) a WHERE a->>'state'='approved') THEN RAISE EXCEPTION 'Retain approved Finance budget bindings';END IF;END $$;
CREATE OR REPLACE FUNCTION protect_quotation_approval_scope() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE a jsonb;n jsonb;l jsonb;nl jsonb;src jsonb;k text;BEGIN
 IF TG_OP='UPDATE' AND (NEW.bo_id IS DISTINCT FROM OLD.bo_id OR NEW.project_id IS DISTINCT FROM OLD.project_id OR NEW.created_at IS DISTINCT FROM OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1) THEN RAISE EXCEPTION 'Quotation approval ownership/version is immutable';END IF;
 IF TG_OP='UPDATE' THEN
  FOR a IN SELECT value FROM jsonb_array_elements(OLD.attempts) LOOP
   SELECT value INTO n FROM jsonb_array_elements(NEW.attempts) WHERE value->>'id'=a->>'id';
   IF NOT FOUND THEN RAISE EXCEPTION 'Quotation submissions must be retained';END IF;
   IF a->>'state'<>'pending_accounts' AND n IS DISTINCT FROM a THEN RAISE EXCEPTION 'Decided quotations are immutable';END IF;
   IF jsonb_array_length(coalesce(a->'proposals','[]'))=0 THEN
    IF n-ARRAY['state','lines','reviewed_by_id','reviewed_at','decision_id','return_action','review_comment'] IS DISTINCT FROM a-ARRAY['state','lines','reviewed_by_id','reviewed_at','decision_id','return_action','review_comment'] THEN RAISE EXCEPTION 'Submitted quotation source is immutable';END IF;
    src:=a;
   ELSE
    IF n-ARRAY['state','lines','reviewed_by_id','reviewed_at','decision_id','return_action','review_comment','quote_key','document_id','revision','vendor_id','vendor','reference','total','verification_fingerprint','terms','valid_until'] IS DISTINCT FROM a-ARRAY['state','lines','reviewed_by_id','reviewed_at','decision_id','return_action','review_comment','quote_key','document_id','revision','vendor_id','vendor','reference','total','verification_fingerprint','terms','valid_until'] THEN RAISE EXCEPTION 'Submitted shortlist and executive reasons are immutable';END IF;
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
    IF NOT FOUND OR nl-ARRAY['decision','comment'] IS DISTINCT FROM l-ARRAY['decision','comment'] THEN RAISE EXCEPTION 'Submitted item source is immutable';END IF;
   END LOOP;
  END LOOP;
 END IF;
 IF NOT EXISTS(SELECT 1 FROM purchase_bo_lists WHERE id=NEW.bo_id AND project_id=NEW.project_id) THEN RAISE EXCEPTION 'Quotation approval BO belongs to another project';END IF;
 RETURN NEW;END $$;
COMMIT;
