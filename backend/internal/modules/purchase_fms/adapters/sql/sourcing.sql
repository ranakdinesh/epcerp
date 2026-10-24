-- name: SourcingGetCase :one
SELECT * FROM purchase_sourcing_cases WHERE project_id= @project_id AND package_id= @package_id;
-- name: SourcingInsertCase :one
INSERT INTO purchase_sourcing_cases(id,project_id,package_id,approver_id,version,state,is_active,source_snapshot,data,created_at,updated_at,created_by_id,updated_by_id)VALUES(@id,@project_id,@package_id,@approver_id,1,@state,true,@source_snapshot,@data,@created_at,@created_at,@actor_id,@actor_id)RETURNING *;
-- name: SourcingUpdateCase :one
UPDATE purchase_sourcing_cases SET approver_id= @approver_id,version=version+1,state= @state,data= @data,updated_at= @updated_at,updated_by_id= @actor_id WHERE project_id= @project_id AND package_id= @package_id AND version= @expected_version RETURNING *;
-- name: SourcingEvent :exec
INSERT INTO purchase_sourcing_events(id,subject_id,action,reason,before_snapshot,after_snapshot,created_at,created_by_id)VALUES(@id,@subject_id,@action,@reason,@before_snapshot,@after_snapshot,@created_at,@actor_id);
-- name: SourcingHistory :many
SELECT e.* FROM purchase_sourcing_events e JOIN purchase_sourcing_cases c ON c.id=e.subject_id WHERE c.project_id= @project_id AND c.package_id= @package_id ORDER BY e.created_at DESC,e.id LIMIT 200;
-- name: SourcingLockAllocation :exec
SELECT pg_advisory_xact_lock(hashtextextended(@lock_key::text,0));
-- name: SourcingAllocationBalances :one
SELECT coalesce(sum(quantity),0)::text AS quantity,coalesce(sum(amount) FILTER(WHERE financial_reference= @financial_reference::text),0)::text AS amount FROM purchase_sourcing_allocations WHERE project_id= @project_id AND demand_id= @demand_id AND is_active AND case_id<> @case_id;
-- name: SourcingReserveAllocation :exec
INSERT INTO purchase_sourcing_allocations(case_id,project_id,demand_id,financial_reference,currency,quantity,amount,approved_quantity,approved_amount,state,case_version,is_active,created_at,updated_at,created_by_id,updated_by_id)
VALUES(@case_id,@project_id,@demand_id,@financial_reference,@currency,@quantity::numeric,@amount::numeric,@approved_quantity::numeric,@approved_amount::numeric,@state,@case_version,true,@at,@at,@actor_id,@actor_id)
ON CONFLICT(case_id)DO UPDATE SET financial_reference=EXCLUDED.financial_reference,currency=EXCLUDED.currency,quantity=EXCLUDED.quantity,amount=EXCLUDED.amount,approved_quantity=EXCLUDED.approved_quantity,approved_amount=EXCLUDED.approved_amount,state=EXCLUDED.state,case_version=EXCLUDED.case_version,is_active=true,updated_at=EXCLUDED.updated_at,updated_by_id=EXCLUDED.updated_by_id;
-- name: SourcingCancelAllocation :exec
UPDATE purchase_sourcing_allocations SET state='cancelled',is_active=false,case_version= @case_version,updated_at= @at,updated_by_id= @actor_id WHERE case_id= @case_id;
