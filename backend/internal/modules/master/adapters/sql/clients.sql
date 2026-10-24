-- name: ClientsInsertClient :exec
INSERT INTO clients (id,client_code,legal_name,display_name,parent_organization,client_type,industry,gstin,pan,cin,website,address_line_1,address_line_2,city,state,postal_code,country,normalized_legal_name,is_active,created_by_id,updated_by_id) VALUES (sqlc.arg(id),sqlc.arg(client_code),sqlc.arg(legal_name),sqlc.arg(display_name),sqlc.arg(parent_organization),sqlc.arg(client_type),sqlc.arg(industry),sqlc.arg(gstin),sqlc.arg(pan),sqlc.arg(cin),sqlc.arg(website),sqlc.arg(address_line_1),sqlc.arg(address_line_2),sqlc.arg(city),sqlc.arg(state),sqlc.arg(postal_code),sqlc.arg(country),sqlc.arg(normalized_legal_name),sqlc.arg(is_active),sqlc.arg(created_by_id),sqlc.arg(updated_by_id));
-- name: ClientsUpdateClient :exec
UPDATE clients SET legal_name=sqlc.arg(legal_name),display_name=sqlc.arg(display_name),parent_organization=sqlc.arg(parent_organization),client_type=sqlc.arg(client_type),industry=sqlc.arg(industry),gstin=sqlc.arg(gstin),pan=sqlc.arg(pan),cin=sqlc.arg(cin),website=sqlc.arg(website),address_line_1=sqlc.arg(address_line_1),address_line_2=sqlc.arg(address_line_2),city=sqlc.arg(city),state=sqlc.arg(state),postal_code=sqlc.arg(postal_code),country=sqlc.arg(country),normalized_legal_name=sqlc.arg(normalized_legal_name),is_active=sqlc.arg(is_active),updated_by_id=sqlc.arg(updated_by_id),updated_at=now() WHERE id=sqlc.arg(id);
-- name: ClientsGetClient :one
SELECT id,client_code,legal_name,normalized_legal_name,display_name,parent_organization,client_type,industry,gstin,pan,cin,website,address_line_1,address_line_2,city,state,postal_code,country,is_active,created_at,created_by_id,updated_at,updated_by_id FROM clients WHERE id=$1;
-- name: ClientsLockClient :one
SELECT id,client_code,legal_name,normalized_legal_name,display_name,parent_organization,client_type,industry,gstin,pan,cin,website,address_line_1,address_line_2,city,state,postal_code,country,is_active,created_at,created_by_id,updated_at,updated_by_id FROM clients WHERE id=$1 FOR UPDATE;
-- name: ClientsClientContacts :many
SELECT * FROM client_contacts WHERE client_id=$1 ORDER BY full_name;
-- name: ClientsListClients :many
SELECT id,client_code,legal_name,normalized_legal_name,display_name,parent_organization,client_type,industry,gstin,pan,cin,website,address_line_1,address_line_2,city,state,postal_code,country,is_active,created_at,created_by_id,updated_at,updated_by_id FROM clients WHERE (sqlc.arg(search)::text='' OR lower(legal_name) LIKE sqlc.arg(search) OR lower(display_name) LIKE sqlc.arg(search) OR lower(client_code) LIKE sqlc.arg(search) OR lower(city) LIKE sqlc.arg(search)) AND (sqlc.narg(active)::boolean IS NULL OR is_active=sqlc.narg(active)) AND (sqlc.arg(client_type)::text='' OR lower(client_type)=sqlc.arg(client_type)) ORDER BY CASE WHEN sqlc.arg(sort)::text='legal_name' AND sqlc.arg(descending)::boolean=false THEN legal_name END ASC,CASE WHEN sqlc.arg(sort)::text='legal_name' AND sqlc.arg(descending)::boolean=true THEN legal_name END DESC,CASE WHEN sqlc.arg(sort)::text='client_code' AND sqlc.arg(descending)::boolean=false THEN client_code END ASC,CASE WHEN sqlc.arg(sort)::text='client_code' AND sqlc.arg(descending)::boolean=true THEN client_code END DESC,CASE WHEN sqlc.arg(sort)::text='created_at' AND sqlc.arg(descending)::boolean=false THEN created_at END ASC,CASE WHEN sqlc.arg(sort)::text='created_at' AND sqlc.arg(descending)::boolean=true THEN created_at END DESC LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);
-- name: ClientsCountClients :one
SELECT count(*) FROM clients WHERE (sqlc.arg(search)::text='' OR lower(legal_name) LIKE sqlc.arg(search) OR lower(display_name) LIKE sqlc.arg(search) OR lower(client_code) LIKE sqlc.arg(search) OR lower(city) LIKE sqlc.arg(search)) AND (sqlc.narg(active)::boolean IS NULL OR is_active=sqlc.narg(active)) AND (sqlc.arg(client_type)::text='' OR lower(client_type)=sqlc.arg(client_type));
-- name: ClientsSummary :one
SELECT count(*) AS all_count,count(*) FILTER(WHERE is_active) AS active_count,count(*) FILTER(WHERE NOT is_active) AS inactive_count FROM clients;
-- name: ClientsRetireContacts :exec
UPDATE client_contacts SET is_active=false,updated_by_id=sqlc.arg(actor_id),updated_at=now() WHERE client_id=sqlc.arg(client_id) AND NOT (id=ANY(sqlc.arg(retained_ids)::uuid[]));
-- name: ClientsInsertContact :exec
INSERT INTO client_contacts (id,client_id,full_name,contact_type,designation,department,email,phone,is_primary,is_active,created_by_id,updated_by_id) VALUES (sqlc.arg(id),sqlc.arg(client_id),sqlc.arg(full_name),sqlc.arg(contact_type),sqlc.arg(designation),sqlc.arg(department),sqlc.arg(email),sqlc.arg(phone),sqlc.arg(is_primary),sqlc.arg(is_active),sqlc.arg(created_by_id),sqlc.arg(updated_by_id));
-- name: ClientsUpdateContact :exec
UPDATE client_contacts SET full_name=sqlc.arg(full_name),contact_type=sqlc.arg(contact_type),designation=sqlc.arg(designation),department=sqlc.arg(department),email=sqlc.arg(email),phone=sqlc.arg(phone),is_primary=sqlc.arg(is_primary),is_active=sqlc.arg(is_active),updated_by_id=sqlc.arg(updated_by_id),updated_at=now() WHERE id=sqlc.arg(id) AND client_id=sqlc.arg(client_id);

-- name: ClientsClientIdentity :one
SELECT id,display_name,client_code,is_active FROM clients WHERE id=$1 FOR SHARE;
-- name: ClientsClientIdentities :many
SELECT id,display_name,client_code,is_active FROM clients WHERE is_active ORDER BY display_name;

