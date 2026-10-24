-- name: GetProject :one
SELECT * FROM projects WHERE id=$1;
-- name: LockProject :one
SELECT * FROM projects WHERE id=$1 FOR UPDATE;
-- name: ListProjects :many
SELECT * FROM projects WHERE (sqlc.narg(manager_id)::uuid IS NULL OR project_manager_id=sqlc.narg(manager_id) OR id=ANY(sqlc.arg(assigned_ids)::uuid[]))
AND (sqlc.arg(search)::text='' OR lower(name) LIKE sqlc.arg(search) OR lower(project_number) LIKE sqlc.arg(search))
ORDER BY created_at DESC,id LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);
-- name: CountProjects :one
SELECT count(*) FROM projects WHERE (sqlc.narg(manager_id)::uuid IS NULL OR project_manager_id=sqlc.narg(manager_id) OR id=ANY(sqlc.arg(assigned_ids)::uuid[]))
AND (sqlc.arg(search)::text='' OR lower(name) LIKE sqlc.arg(search) OR lower(project_number) LIKE sqlc.arg(search));
-- name: CreateProject :one
WITH number AS (SELECT nextval('project_number_sequence') AS value)
INSERT INTO projects(id,project_number,name,normalized_name,client_id,project_manager_id,created_by_id,updated_by_id,short_name,project_type,site_name,site_state,site_city,scope_summary,operations_manager_id,contract_value,currency,loi_number,po_number,schedule_start_basis,planned_start_date,planned_completion_date,loi_date,po_date,actual_start_date,site_handover_date,planned_duration_days,safety_buffer_percent,safety_buffer_days,remarks,actual_end_date,schedule_calculation,purchase_enabled)
SELECT sqlc.arg(id),'PRJ-' || CASE WHEN value<1000000 THEN lpad(value::text,6,'0') ELSE value::text END,sqlc.arg(name),sqlc.arg(normalized_name),sqlc.arg(client_id),sqlc.arg(project_manager_id),sqlc.arg(actor_id),sqlc.arg(actor_id),sqlc.arg(short_name)::text,sqlc.arg(project_type)::text,sqlc.arg(site_name)::text,sqlc.arg(site_state)::text,sqlc.arg(site_city)::text,sqlc.arg(scope_summary)::text,NULLIF(sqlc.arg(operations_manager_id)::text,'')::uuid,NULLIF(sqlc.arg(contract_value)::text,'')::numeric(16,2),sqlc.arg(currency)::text,sqlc.arg(loi_number)::text,sqlc.arg(po_number)::text,sqlc.arg(schedule_start_basis)::text,NULLIF(sqlc.arg(planned_start_date)::text,'')::date,NULLIF(sqlc.arg(planned_completion_date)::text,'')::date,NULLIF(sqlc.arg(loi_date)::text,'')::date,NULLIF(sqlc.arg(po_date)::text,'')::date,NULLIF(sqlc.arg(actual_start_date)::text,'')::date,NULLIF(sqlc.arg(site_handover_date)::text,'')::date,NULLIF(sqlc.arg(planned_duration_days)::text,'')::integer,NULLIF(sqlc.arg(safety_buffer_percent)::text,'')::numeric(5,2),NULLIF(sqlc.arg(safety_buffer_days)::text,'')::integer,sqlc.arg(remarks)::text,NULLIF(sqlc.arg(actual_end_date)::text,'')::date,sqlc.arg(schedule_calculation)::jsonb,sqlc.arg(purchase_enabled)::boolean FROM number RETURNING *;
-- name: TransferManager :one
UPDATE projects SET project_manager_id=sqlc.arg(manager_id),version=version+1,updated_at=clock_timestamp(),updated_by_id=sqlc.arg(actor_id)
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) RETURNING *;
-- name: InsertManagerHistory :exec
INSERT INTO project_manager_history(id,project_id,previous_manager_id,project_manager_id,project_version,reason,created_at,created_by_id)
VALUES(sqlc.arg(id),sqlc.arg(project_id),sqlc.narg(previous_manager_id),sqlc.arg(manager_id),sqlc.arg(version),sqlc.arg(reason),sqlc.arg(occurred_at),sqlc.arg(actor_id));
-- name: ManagerHistory :many
SELECT * FROM project_manager_history WHERE project_id=$1 ORDER BY project_version DESC;
-- name: ListProcessProjects :many
SELECT * FROM projects WHERE (sqlc.arg(portfolio)::boolean OR project_manager_id=sqlc.narg(actor_id)::uuid OR id=ANY(sqlc.arg(assigned_ids)::uuid[]))
AND (sqlc.arg(search)::text='' OR lower(name) LIKE sqlc.arg(search) OR lower(project_number) LIKE sqlc.arg(search))
ORDER BY
 CASE WHEN sqlc.arg(direction)::text='asc' AND sqlc.arg(sort)::text='number' THEN project_number END ASC,
 CASE WHEN sqlc.arg(direction)::text='desc' AND sqlc.arg(sort)::text='number' THEN project_number END DESC,
 CASE WHEN sqlc.arg(direction)::text='asc' AND sqlc.arg(sort)::text='name' THEN lower(name) END ASC,
 CASE WHEN sqlc.arg(direction)::text='desc' AND sqlc.arg(sort)::text='name' THEN lower(name) END DESC,
 CASE WHEN sqlc.arg(direction)::text='asc' AND sqlc.arg(sort)::text='state' THEN lifecycle_status END ASC,
 CASE WHEN sqlc.arg(direction)::text='desc' AND sqlc.arg(sort)::text='state' THEN lifecycle_status END DESC,
 id LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);
-- name: CountProcessProjects :one
SELECT count(*) FROM projects WHERE (sqlc.arg(portfolio)::boolean OR project_manager_id=sqlc.narg(actor_id)::uuid OR id=ANY(sqlc.arg(assigned_ids)::uuid[]))
AND (sqlc.arg(search)::text='' OR lower(name) LIKE sqlc.arg(search) OR lower(project_number) LIKE sqlc.arg(search));

-- name: RecalculationProjects :many
SELECT * FROM projects WHERE actual_start_date IS NOT NULL AND planned_completion_date IS NOT NULL ORDER BY id FOR UPDATE;
-- name: SaveCalculation :exec
UPDATE projects SET schedule_calculation=sqlc.arg(calculation),planned_duration_days=sqlc.arg(days),safety_buffer_days=sqlc.arg(buffer),version=version+1,updated_by_id=sqlc.arg(actor),updated_at=clock_timestamp() WHERE id=sqlc.arg(id);

-- name: CalculationHistory :exec
INSERT INTO project_schedule_changes(id,project_id,project_version,before_calculation,after_calculation,created_by_id)
SELECT sqlc.arg(event_id),p.id,p.version+1,p.schedule_calculation,sqlc.arg(calculation)::jsonb,sqlc.arg(actor)::uuid FROM projects p WHERE p.id=sqlc.arg(project_id);



-- name: UpdateMetadata :one
UPDATE projects SET name=sqlc.arg(name),normalized_name=sqlc.arg(normalized_name),client_id=sqlc.arg(client_id),short_name=sqlc.arg(short_name)::text,project_type=sqlc.arg(project_type)::text,site_name=sqlc.arg(site_name)::text,site_state=sqlc.arg(site_state)::text,site_city=sqlc.arg(site_city)::text,scope_summary=sqlc.arg(scope_summary)::text,operations_manager_id=NULLIF(sqlc.arg(operations_manager_id)::text,'')::uuid,contract_value=NULLIF(sqlc.arg(contract_value)::text,'')::numeric(16,2),currency=sqlc.arg(currency)::text,loi_number=sqlc.arg(loi_number)::text,po_number=sqlc.arg(po_number)::text,schedule_start_basis=sqlc.arg(schedule_start_basis)::text,planned_start_date=NULLIF(sqlc.arg(planned_start_date)::text,'')::date,planned_completion_date=NULLIF(sqlc.arg(planned_completion_date)::text,'')::date,loi_date=NULLIF(sqlc.arg(loi_date)::text,'')::date,po_date=NULLIF(sqlc.arg(po_date)::text,'')::date,actual_start_date=NULLIF(sqlc.arg(actual_start_date)::text,'')::date,site_handover_date=NULLIF(sqlc.arg(site_handover_date)::text,'')::date,planned_duration_days=NULLIF(sqlc.arg(planned_duration_days)::text,'')::integer,safety_buffer_percent=NULLIF(sqlc.arg(safety_buffer_percent)::text,'')::numeric(5,2),safety_buffer_days=NULLIF(sqlc.arg(safety_buffer_days)::text,'')::integer,remarks=sqlc.arg(remarks)::text,actual_end_date=NULLIF(sqlc.arg(actual_end_date)::text,'')::date,schedule_calculation=sqlc.arg(schedule_calculation)::jsonb,purchase_enabled=sqlc.arg(purchase_enabled)::boolean,version=version+1,updated_by_id=sqlc.arg(actor_id),updated_at=clock_timestamp() WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) RETURNING *;

-- name: InsertMetadataHistory :exec
INSERT INTO project_metadata_changes(id,project_id,project_version,reason,before_snapshot,after_snapshot,created_by_id) VALUES(sqlc.arg(id),sqlc.arg(project_id),sqlc.arg(version),sqlc.arg(reason),sqlc.arg(before_snapshot),sqlc.arg(after_snapshot),sqlc.arg(actor));
-- name: MetadataHistory :many
SELECT * FROM project_metadata_changes WHERE project_id=$1 ORDER BY project_version DESC;

-- name: ReadWorkInventory :many
WITH assignments AS (
SELECT 'project_manager'::text AS kind,p.id::text AS id,p.project_manager_id::text AS owner_id,p.version::bigint AS version,p.project_number||' · '||p.name::text AS title,p.lifecycle_status::text AS state,'/dashboard?screen=projects&project='||p.id::text::text AS link,jsonb_build_array('projects.master.read','projects.master.write')::jsonb AS required_permissions,''::text AS due_date FROM projects p WHERE p.is_active AND p.lifecycle_status<>'closed'
) SELECT jsonb_build_object('Source','projects','Kind',kind,'ID',id,'OwnerID',owner_id,'Version',version,'Title',title,'State',state,'Link',link,'RequiredPermissions',required_permissions,'DueDate',due_date)::jsonb AS payload FROM assignments WHERE (sqlc.arg(owner_id)::text='' OR owner_id=sqlc.arg(owner_id)) AND (sqlc.arg(record_id)::text='' OR id=sqlc.arg(record_id)) AND (sqlc.arg(kind)::text='' OR kind=sqlc.arg(kind)) ORDER BY kind,id,owner_id LIMIT 10001;
