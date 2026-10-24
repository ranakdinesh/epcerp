BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM quotation_approval_sets q CROSS JOIN LATERAL jsonb_array_elements(q.attempts) a WHERE jsonb_array_length(coalesce(a->'proposals','[]'))>0) THEN RAISE EXCEPTION 'Retain populated shortlist history';END IF;END $$;
DROP TRIGGER quotation_shortlist_sources ON quotation_approval_sets;
DROP FUNCTION check_quotation_shortlist_sources();
CREATE OR REPLACE FUNCTION protect_quotation_approval_scope() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE a jsonb;n jsonb;l jsonb;nl jsonb;BEGIN
 IF TG_OP='UPDATE' AND (NEW.bo_id IS DISTINCT FROM OLD.bo_id OR NEW.project_id IS DISTINCT FROM OLD.project_id OR NEW.created_at IS DISTINCT FROM OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1) THEN RAISE EXCEPTION 'Quotation approval ownership/version is immutable';END IF;
 IF TG_OP='UPDATE' THEN
  FOR a IN SELECT value FROM jsonb_array_elements(OLD.attempts) LOOP
   SELECT value INTO n FROM jsonb_array_elements(NEW.attempts) WHERE value->>'id'=a->>'id';
   IF NOT FOUND THEN RAISE EXCEPTION 'Quotation submissions must be retained';END IF;
   IF a->>'state'<>'pending_accounts' AND n IS DISTINCT FROM a THEN RAISE EXCEPTION 'Decided quotations are immutable';END IF;
   IF n-ARRAY['state','lines','reviewed_by_id','reviewed_at','decision_id'] IS DISTINCT FROM a-ARRAY['state','lines','reviewed_by_id','reviewed_at','decision_id'] THEN RAISE EXCEPTION 'Submitted quotation source is immutable';END IF;
   IF jsonb_array_length(n->'lines')<>jsonb_array_length(a->'lines') THEN RAISE EXCEPTION 'Every submitted item must be retained';END IF;
   FOR l IN SELECT value FROM jsonb_array_elements(a->'lines') LOOP
    SELECT value INTO nl FROM jsonb_array_elements(n->'lines') WHERE value->>'line_id'=l->>'line_id';
    IF NOT FOUND OR nl-ARRAY['decision','comment'] IS DISTINCT FROM l-ARRAY['decision','comment'] THEN RAISE EXCEPTION 'Submitted item source is immutable';END IF;
   END LOOP;
  END LOOP;
 END IF;
 IF NOT EXISTS(SELECT 1 FROM purchase_bo_lists WHERE id=NEW.bo_id AND project_id=NEW.project_id) THEN RAISE EXCEPTION 'Quotation approval BO belongs to another project';END IF;
 RETURN NEW;END $$;
COMMIT;
