-- name: GetBudget :one
SELECT * FROM project_budgets WHERE project_id= @project_id;
-- name: BudgetHeads :many
SELECT * FROM project_budget_heads WHERE project_id= @project_id ORDER BY sort_order,id;
-- name: BudgetAllocations :many
SELECT * FROM project_budget_allocations WHERE project_id= @project_id ORDER BY sort_order,id;
-- name: InsertBudget :one
INSERT INTO project_budgets(project_id,version,revision,state,total,currency,owner_id,notes,pricing_mode,is_active,created_by_id,updated_by_id)VALUES(@project_id,1,@revision,'draft',@total,@currency,@owner_id,@notes,@pricing_mode,true,@actor_id,@actor_id) RETURNING *;
-- name: UpdateBudget :one
UPDATE project_budgets SET version=version+1,revision= @revision,state= @state,approved_snapshot= @approved_snapshot,approval_decision_id=sqlc.narg(approval_decision_id),total= @total,currency= @currency,owner_id= @owner_id,notes= @notes,pricing_mode= @pricing_mode,is_active= @is_active,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE project_id= @project_id AND version= @expected_version RETURNING *;
-- name: SaveBudgetHead :execrows
INSERT INTO project_budget_heads(id,project_id,sort_order,parent_id,name,kind,amount,owner_id,is_active,created_at,updated_at,created_by_id,updated_by_id)VALUES(@id,@project_id,@sort_order,sqlc.narg(parent_id),@name,@kind,@amount,@owner_id,@is_active,@created_at,@updated_at,sqlc.narg(created_by_id),@actor_id)
ON CONFLICT(id) DO UPDATE SET sort_order=EXCLUDED.sort_order,parent_id=EXCLUDED.parent_id,name=EXCLUDED.name,kind=EXCLUDED.kind,amount=EXCLUDED.amount,owner_id=EXCLUDED.owner_id,is_active=EXCLUDED.is_active,updated_at=EXCLUDED.updated_at,updated_by_id=EXCLUDED.updated_by_id WHERE project_budget_heads.project_id=EXCLUDED.project_id;
-- name: SaveBudgetAllocation :execrows
INSERT INTO project_budget_allocations(id,project_id,sort_order,bo_id,line_id,head_id,amount,owner_id,is_active,unit_price,pricing_quantity,source_snapshot,created_at,updated_at,created_by_id,updated_by_id)VALUES(@id,@project_id,@sort_order,@bo_id,@line_id,@head_id,@amount,@owner_id,@is_active,sqlc.narg(unit_price),sqlc.narg(pricing_quantity),@source_snapshot,@created_at,@updated_at,sqlc.narg(created_by_id),@actor_id)
ON CONFLICT(id) DO UPDATE SET sort_order=EXCLUDED.sort_order,head_id=EXCLUDED.head_id,amount=EXCLUDED.amount,owner_id=EXCLUDED.owner_id,is_active=EXCLUDED.is_active,unit_price=EXCLUDED.unit_price,pricing_quantity=EXCLUDED.pricing_quantity,source_snapshot=EXCLUDED.source_snapshot,updated_at=EXCLUDED.updated_at,updated_by_id=EXCLUDED.updated_by_id WHERE project_budget_allocations.project_id=EXCLUDED.project_id AND project_budget_allocations.bo_id=EXCLUDED.bo_id AND project_budget_allocations.line_id=EXCLUDED.line_id;
-- name: InsertBudgetEvent :exec
INSERT INTO project_budget_events(id,project_id,action,reason,before_snapshot,after_snapshot,created_by_id)VALUES(@id,@project_id,@action,@reason,@before_snapshot,@after_snapshot,@actor_id);
-- name: BudgetHistory :many
SELECT * FROM project_budget_events WHERE project_id= @project_id ORDER BY created_at DESC,id DESC LIMIT 200;

-- name: ReadWorkInventory :many
WITH assignments AS (
SELECT 'budget'::text AS kind,p.project_id::text AS id,p.owner_id::text AS owner_id,p.version::bigint AS version,'Budget'::text AS title,'active'::text AS state,'/dashboard?screen=projects&project='||p.project_id::text::text AS link,jsonb_build_array('projects.budgets.write')::jsonb AS required_permissions,''::text AS due_date, p.project_id::text AS project_id FROM project_budgets p  WHERE p.is_active AND p.state='draft'
UNION ALL
SELECT 'budget_head'::text AS kind,p.id::text AS id,p.owner_id::text AS owner_id,b.version::bigint AS version,'Budget head'::text AS title,'active'::text AS state,'/dashboard?screen=projects&project='||p.project_id::text::text AS link,jsonb_build_array('projects.budgets.write')::jsonb AS required_permissions,''::text AS due_date, p.project_id::text AS project_id FROM project_budget_heads p JOIN project_budgets b ON b.project_id=p.project_id WHERE p.is_active AND b.is_active AND b.state='draft'
UNION ALL
SELECT 'budget_allocation'::text AS kind,p.id::text AS id,p.owner_id::text AS owner_id,b.version::bigint AS version,'Budget allocation'::text AS title,'active'::text AS state,'/dashboard?screen=projects&project='||p.project_id::text::text AS link,jsonb_build_array('projects.budgets.write')::jsonb AS required_permissions,''::text AS due_date, p.project_id::text AS project_id FROM project_budget_allocations p JOIN project_budgets b ON b.project_id=p.project_id WHERE p.is_active AND b.is_active AND b.state='draft'
) SELECT jsonb_build_object('Source','project_budgets','Kind',kind,'ID',id,'OwnerID',owner_id,'Version',version,'Title',title,'State',state,'Link',link,'RequiredPermissions',required_permissions,'DueDate',due_date,'ProjectID',project_id)::jsonb AS payload FROM assignments WHERE (sqlc.arg(owner_id)::text='' OR owner_id=sqlc.arg(owner_id)) AND (sqlc.arg(record_id)::text='' OR id=sqlc.arg(record_id)) AND (sqlc.arg(kind)::text='' OR kind=sqlc.arg(kind)) ORDER BY kind,id,owner_id LIMIT 10001;
