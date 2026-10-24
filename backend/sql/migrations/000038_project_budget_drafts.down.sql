BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM project_budgets) OR EXISTS(SELECT 1 FROM project_budget_events) THEN RAISE EXCEPTION 'Refuse to remove populated budget draft evidence';END IF;END $$;
DROP TABLE project_budget_events,project_budget_allocations,project_budget_heads,project_budgets;
DROP FUNCTION protect_project_budget_item_scope(),protect_project_budget_child_identity(),protect_project_budget_identity(),protect_project_budget_evidence();
COMMIT;
