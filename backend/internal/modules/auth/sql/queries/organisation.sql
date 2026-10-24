-- name: OrganisationDepartments :many
SELECT * FROM auth.auth_departments ORDER BY name,id;
-- name: OrganisationEmployees :many
SELECT u.id,u.username,u.email,u.first_name,u.last_name,u.mobile,u.is_active,r.department_id,r.manager_id,COALESCE(r.is_company_root,false)::boolean AS is_company_root,COALESCE(r.version,0)::bigint AS version
FROM auth.users u LEFT JOIN auth.auth_reporting_lines r ON r.user_id=u.id AND r.is_active ORDER BY u.username,u.id;
-- name: SaveOrganisationDepartment :one
INSERT INTO auth.auth_departments(id,code,name,manager_id,is_active,created_by_id,updated_by_id)
VALUES(sqlc.arg(id),sqlc.arg(code),sqlc.arg(name),sqlc.narg(manager_id),sqlc.arg(is_active),sqlc.arg(actor_id),sqlc.arg(actor_id))
ON CONFLICT(id) DO UPDATE SET code=excluded.code,name=excluded.name,manager_id=excluded.manager_id,is_active=excluded.is_active,version=auth_departments.version+1,updated_at=clock_timestamp(),updated_by_id=excluded.updated_by_id
WHERE auth_departments.version=sqlc.arg(expected_version) RETURNING *;
-- name: SaveOrganisationEmployee :one
INSERT INTO auth.auth_reporting_lines(user_id,department_id,manager_id,is_company_root,created_by_id,updated_by_id)
VALUES(sqlc.arg(user_id),sqlc.arg(department_id),sqlc.narg(manager_id),sqlc.arg(is_company_root),sqlc.arg(actor_id),sqlc.arg(actor_id))
ON CONFLICT(user_id) DO UPDATE SET department_id=excluded.department_id,manager_id=excluded.manager_id,is_company_root=excluded.is_company_root,version=auth_reporting_lines.version+1,updated_at=clock_timestamp(),updated_by_id=excluded.updated_by_id
WHERE auth_reporting_lines.version=sqlc.arg(expected_version) RETURNING *;
-- name: SaveEmployeeContact :exec
UPDATE auth.users SET first_name=$2,last_name=$3,mobile=$4,updated_by_id=$5,updated_at=clock_timestamp() WHERE id=$1;
-- name: RecordOrganisationDepartmentChange :exec
INSERT INTO auth.auth_organisation_changes(id,entity_id,entity_type,version,reason,snapshot,actor_id)
SELECT sqlc.arg(event_id),d.id,'department',d.version,sqlc.arg(reason),to_jsonb(d),sqlc.arg(actor_id) FROM auth.auth_departments d WHERE d.id=sqlc.arg(entity_id);
-- name: RecordOrganisationEmployeeChange :exec
INSERT INTO auth.auth_organisation_changes(id,entity_id,entity_type,version,reason,snapshot,actor_id)
SELECT sqlc.arg(event_id),r.user_id,'employee',r.version,sqlc.arg(reason),to_jsonb(r)||jsonb_build_object('first_name',u.first_name,'last_name',u.last_name,'mobile',u.mobile),sqlc.arg(actor_id)
FROM auth.auth_reporting_lines r JOIN auth.users u ON u.id=r.user_id WHERE r.user_id=sqlc.arg(entity_id);
