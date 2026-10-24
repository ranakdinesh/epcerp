-- name: WorkflowPackageExists :one
SELECT EXISTS(SELECT 1 FROM purchase_work_packages WHERE project_id= @project_id AND indent_id= @indent_id AND demand_id= @demand_id);
-- name: WorkflowGetPackage :one
SELECT * FROM purchase_work_packages WHERE project_id= @project_id AND id= @id;
-- name: WorkflowInsertPackage :one
INSERT INTO purchase_work_packages(id,project_id,indent_id,demand_id,code,description,source_snapshot,steps,version,created_by_id,updated_by_id)VALUES(@id,@project_id,@indent_id,@demand_id,@code,@description,@source_snapshot,@steps,1,@actor_id,@actor_id)RETURNING *;
-- name: WorkflowUpdatePackage :one
UPDATE purchase_work_packages SET steps= @steps,version=version+1,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE project_id= @project_id AND id= @id AND version= @expected_version RETURNING *;
-- name: WorkflowEvent :exec
INSERT INTO purchase_work_package_events(id,subject_id,action,reason,before_snapshot,after_snapshot,created_at,created_by_id)VALUES(@id,@subject_id,@action,@reason,@before_snapshot,@after_snapshot,@created_at,@actor_id);
-- name: WorkflowHistory :many
SELECT e.* FROM purchase_work_package_events e JOIN purchase_work_packages p ON p.id=e.subject_id WHERE p.project_id= @project_id AND e.subject_id= @id ORDER BY e.created_at DESC,e.id LIMIT 200;
-- name: WorkflowCountPackages :one
SELECT count(*)::bigint FROM purchase_work_packages WHERE project_id= @project_id AND is_active AND (code ILIKE @search::text OR description ILIKE @search::text);
-- name: WorkflowPagePackages :many
SELECT p.id,p.code,p.description,coalesce(p.source_snapshot->>'quantity','')::text AS quantity,coalesce(p.source_snapshot->>'uom','')::text AS uom,p.version,p.updated_at,
 coalesce(s.data->'decision_snapshot'->'offer'->'supplier_snapshot'->>'name','')::text AS supplier,
 coalesce(s.state,'not_started')::text AS loi_state,coalesce(t.state,'not_started')::text AS technical_state,
 coalesce(o.state,'not_started')::text AS order_state,coalesce(o.data->'client_approval'->>'status','not_submitted')::text AS client_po_state
 FROM purchase_work_packages p LEFT JOIN purchase_sourcing_cases s ON s.package_id=p.id AND s.project_id=p.project_id AND s.is_active
 LEFT JOIN purchase_technical_cases t ON t.package_id=p.id AND t.project_id=p.project_id AND t.is_active
 LEFT JOIN purchase_orders o ON o.package_id=p.id AND o.project_id=p.project_id AND o.is_active
 WHERE p.project_id= @project_id AND p.is_active AND (p.code ILIKE @search::text OR p.description ILIKE @search::text)
ORDER BY
 CASE WHEN @sort::text='code' AND @direction::text='asc' THEN p.code END ASC,
 CASE WHEN @sort::text='code' AND @direction::text='desc' THEN p.code END DESC,
 CASE WHEN @sort::text='description' AND @direction::text='asc' THEN p.description END ASC,
 CASE WHEN @sort::text='description' AND @direction::text='desc' THEN p.description END DESC,
 CASE WHEN @sort::text='updated' AND @direction::text='asc' THEN p.updated_at END ASC,
 CASE WHEN @sort::text='updated' AND @direction::text='desc' THEN p.updated_at END DESC,p.id
LIMIT @page_size OFFSET @record_offset;

-- name: WorkflowCountAssignedTasks :one
SELECT count(*) FROM purchase_work_packages p CROSS JOIN LATERAL jsonb_array_elements(p.steps) s WHERE p.is_active AND (p.steps @> jsonb_build_array(jsonb_build_object('doer_id',@actor_id::text)) OR p.steps @> jsonb_build_array(jsonb_build_object('blocked_resolver_id',@actor_id::text))) AND (s->>'number')::integer BETWEEN 3 AND 20
AND ((@can_execute::boolean AND s->>'doer_id'= @actor_id::text AND @role::text IN('all','doer')) OR (@can_resolve::boolean AND s->>'state'='blocked' AND s->>'blocked_resolver_id'= @actor_id::text AND @role::text IN('all','resolver')))
AND (@state::text='all' OR s->>'state'= @state::text)
AND (p.code ILIKE @search::text OR p.description ILIKE @search::text OR s->>'title' ILIKE @search::text);
-- name: WorkflowPageAssignedTasks :many
SELECT p.id,p.project_id,p.code,p.version,s AS step FROM purchase_work_packages p CROSS JOIN LATERAL jsonb_array_elements(p.steps) s WHERE p.is_active AND (p.steps @> jsonb_build_array(jsonb_build_object('doer_id',@actor_id::text)) OR p.steps @> jsonb_build_array(jsonb_build_object('blocked_resolver_id',@actor_id::text))) AND (s->>'number')::integer BETWEEN 3 AND 20
AND ((@can_execute::boolean AND s->>'doer_id'= @actor_id::text AND @role::text IN('all','doer')) OR (@can_resolve::boolean AND s->>'state'='blocked' AND s->>'blocked_resolver_id'= @actor_id::text AND @role::text IN('all','resolver')))
AND (@state::text='all' OR s->>'state'= @state::text)
AND (p.code ILIKE @search::text OR p.description ILIKE @search::text OR s->>'title' ILIKE @search::text)
ORDER BY CASE WHEN s->>'state'='blocked' THEN 0 ELSE 1 END,coalesce(nullif(s->>'planned_date',''),'9999-12-31'),p.id,(s->>'number')::integer LIMIT @page_size OFFSET @record_offset;

-- name: WorkflowCountProgressPackages :one
SELECT count(*) FROM purchase_work_packages WHERE project_id= @project_id AND is_active AND demand_id= ANY(@demand_ids::uuid[]);
-- name: WorkflowPageProgressPackages :many
SELECT * FROM purchase_work_packages WHERE project_id= @project_id AND is_active AND demand_id= ANY(@demand_ids::uuid[]) ORDER BY id LIMIT @page_size OFFSET @record_offset;
