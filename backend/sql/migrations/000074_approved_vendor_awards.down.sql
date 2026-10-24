BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM purchase_award_handoffs) THEN RAISE EXCEPTION 'Retain recorded award commitments';END IF;END $$;
DROP TABLE purchase_award_handoffs;
DROP FUNCTION check_award_handoff();
ALTER TABLE purchase_sourcing_events DROP CONSTRAINT purchase_sourcing_events_action_check;
ALTER TABLE purchase_sourcing_events ADD CONSTRAINT purchase_sourcing_events_action_check CHECK(action IN('configure','rfq','dispatch','no_bid','quote','propose','decide','reopen','release'));
CREATE OR REPLACE FUNCTION protect_purchase_sourcing() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE o jsonb;n jsonb;BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_work_packages p WHERE p.id=NEW.package_id AND p.project_id=NEW.project_id) THEN RAISE EXCEPTION 'Sourcing package belongs to another project';END IF;
 IF NEW.data->>'id' IS DISTINCT FROM NEW.id::text OR NEW.data->>'project_id' IS DISTINCT FROM NEW.project_id::text OR NEW.data->>'package_id' IS DISTINCT FROM NEW.package_id::text OR NEW.data->>'approver_id' IS DISTINCT FROM NEW.approver_id::text OR NEW.data->>'state' IS DISTINCT FROM NEW.state OR NEW.data->>'version' IS DISTINCT FROM NEW.version::text OR NEW.data->'source_snapshot' IS DISTINCT FROM NEW.source_snapshot OR NEW.data->>'is_active' IS DISTINCT FROM NEW.is_active::text THEN RAISE EXCEPTION 'Sourcing record projection does not match owned state';END IF;
 IF jsonb_typeof(NEW.data->'offers') IS DISTINCT FROM 'array' OR jsonb_array_length(NEW.data->'offers')>50 THEN RAISE EXCEPTION 'Sourcing offers must be a bounded array';END IF;
 FOR o IN SELECT value FROM jsonb_array_elements(NEW.data->'offers') LOOP
  IF jsonb_typeof(o) IS DISTINCT FROM 'object' OR coalesce(o->>'id','') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$' OR coalesce(o->>'state','') NOT IN('draft','sent','no_bid','quoted') OR o->>'is_active' IS DISTINCT FROM 'true' OR NOT EXISTS(SELECT 1 FROM suppliers WHERE id::text=o->>'supplier_id') THEN RAISE EXCEPTION 'Sourcing offers require owned identity, supplier and state';END IF;
 END LOOP;
 IF EXISTS(SELECT 1 FROM jsonb_array_elements(NEW.data->'offers') GROUP BY value->>'id' HAVING count(*)>1) OR EXISTS(SELECT 1 FROM jsonb_array_elements(NEW.data->'offers') GROUP BY value->>'supplier_id' HAVING count(*)>1) THEN RAISE EXCEPTION 'Sourcing offers must have unique identities and suppliers';END IF;
 IF NEW.state IN('pending','approved','released') AND (jsonb_typeof(NEW.data->'decision_snapshot') IS DISTINCT FROM 'object' OR NEW.data->>'selected_offer_id' IS NULL OR NEW.data->>'selected_offer_id'='' OR NEW.data->>'proposed_by_id' IS NULL OR NEW.data->>'proposed_by_id'='' OR NEW.data->>'proposed_by_id'=NEW.approver_id::text) THEN RAISE EXCEPTION 'Independent sourcing selection evidence is required';END IF;
 IF NEW.state IN('pending','approved','released') AND (NEW.data->'decision_snapshot'->'source' IS DISTINCT FROM NEW.source_snapshot OR NEW.data->'decision_snapshot'->'terms' IS DISTINCT FROM NEW.data->'loi_terms' OR coalesce(NEW.data->'decision_snapshot'->>'financial_reference','')='' OR NOT EXISTS(SELECT 1 FROM jsonb_array_elements(NEW.data->'offers') WHERE value->>'id'=NEW.data->>'selected_offer_id' AND value=NEW.data->'decision_snapshot'->'offer' AND value->>'state'='quoted' AND value->>'compliant'='true')) THEN RAISE EXCEPTION 'Selection must capture its exact source, offer, terms and Finance evidence';END IF;
 IF NEW.state IN('approved','released') AND (NEW.data->>'approved_by_id' IS DISTINCT FROM NEW.approver_id::text OR NEW.data->>'approved_at' IS NULL OR NEW.data->>'approved_by_id'=NEW.data->>'proposed_by_id') THEN RAISE EXCEPTION 'Independent sourcing approval is required';END IF;
 IF NEW.state='released' AND (coalesce(NEW.data->>'loi_reference','')='' OR NEW.data->>'released_at' IS NULL OR coalesce(NEW.data->>'loi_dispatch_reference','')='') THEN RAISE EXCEPTION 'Durable LOI and dispatch evidence are required';END IF;
 IF TG_OP='INSERT' AND NEW.state<>'draft' THEN RAISE EXCEPTION 'Sourcing must begin in draft';END IF;
 IF TG_OP='UPDATE' THEN
  IF NOT ((OLD.state='draft' AND NEW.state IN('draft','pending')) OR (OLD.state='pending' AND NEW.state IN('draft','approved')) OR (OLD.state='approved' AND NEW.state IN('draft','released'))) THEN RAISE EXCEPTION 'Invalid sourcing transition';END IF;
  IF OLD.state='released' THEN RAISE EXCEPTION 'Released LOI is immutable';END IF;
  IF NEW.id IS DISTINCT FROM OLD.id OR NEW.project_id IS DISTINCT FROM OLD.project_id OR NEW.package_id IS DISTINCT FROM OLD.package_id OR NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot OR NEW.created_at IS DISTINCT FROM OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'Sourcing ownership/source/version is immutable';END IF;
  IF OLD.state IN('pending','approved') AND NEW.state<>'draft' AND (NEW.data->'decision_snapshot' IS DISTINCT FROM OLD.data->'decision_snapshot' OR NEW.data->'offers' IS DISTINCT FROM OLD.data->'offers' OR NEW.approver_id IS DISTINCT FROM OLD.approver_id) THEN RAISE EXCEPTION 'Submitted sourcing evidence is immutable';END IF;
  FOR o IN SELECT value FROM jsonb_array_elements(OLD.data->'offers') LOOP
   SELECT value INTO n FROM jsonb_array_elements(NEW.data->'offers') WHERE value->>'id'=o->>'id';
   IF NOT FOUND OR n->>'supplier_id' IS DISTINCT FROM o->>'supplier_id' OR n->>'created_at' IS DISTINCT FROM o->>'created_at' OR n->>'created_by_id' IS DISTINCT FROM o->>'created_by_id' THEN RAISE EXCEPTION 'Sourcing offer ownership must be retained';END IF;
  END LOOP;
 END IF;
 RETURN NEW;END $$;
CREATE OR REPLACE FUNCTION check_purchase_sourcing_reservation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.state<>'draft' AND NOT EXISTS(SELECT 1 FROM purchase_sourcing_allocations WHERE case_id=NEW.id AND is_active AND state=NEW.state AND case_version=NEW.version) THEN RAISE EXCEPTION 'Submitted sourcing requires its owning allocation';END IF;
 IF NEW.state='draft' AND EXISTS(SELECT 1 FROM purchase_sourcing_allocations WHERE case_id=NEW.id AND is_active) THEN RAISE EXCEPTION 'Draft sourcing cannot retain a reservation';END IF;
 RETURN NEW;END $$;
COMMIT;
