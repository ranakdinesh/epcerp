DO $$BEGIN IF EXISTS(SELECT 1 FROM purchase_work_packages)OR EXISTS(SELECT 1 FROM purchase_work_package_events)THEN RAISE EXCEPTION 'Cannot roll back populated purchase work evidence';END IF;END $$;
DROP TABLE purchase_work_package_events;
DROP TABLE purchase_work_packages;
DROP FUNCTION protect_purchase_work_package();
DROP FUNCTION refuse_purchase_work_evidence_mutation();
