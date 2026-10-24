-- name: ReferencesGetReference :one
SELECT id,name,code,normalized_name,is_active,created_at,updated_at,created_by_id,updated_by_id,NULL::uuid AS state_id FROM client_types WHERE client_types.id=sqlc.arg(reference_id) AND sqlc.arg(kind)::text='client-types'
UNION ALL SELECT id,name,code,normalized_name,is_active,created_at,updated_at,created_by_id,updated_by_id,NULL::uuid AS state_id FROM industries WHERE industries.id=sqlc.arg(reference_id) AND sqlc.arg(kind)::text='industries'
UNION ALL SELECT id,name,code,normalized_name,is_active,created_at,updated_at,created_by_id,updated_by_id,NULL::uuid AS state_id FROM indian_states WHERE indian_states.id=sqlc.arg(reference_id) AND sqlc.arg(kind)::text='states'
UNION ALL SELECT id,name,code,normalized_name,is_active,created_at,updated_at,created_by_id,updated_by_id,state_id FROM indian_cities WHERE indian_cities.id=sqlc.arg(reference_id) AND sqlc.arg(kind)::text='cities';
-- name: ReferencesListReferences :many
SELECT * FROM (SELECT id,name,code,normalized_name,is_active,created_at,updated_at,created_by_id,updated_by_id,NULL::uuid AS state_id FROM client_types WHERE sqlc.arg(kind)::text='client-types'
UNION ALL SELECT id,name,code,normalized_name,is_active,created_at,updated_at,created_by_id,updated_by_id,NULL::uuid AS state_id FROM industries WHERE sqlc.arg(kind)::text='industries'
UNION ALL SELECT id,name,code,normalized_name,is_active,created_at,updated_at,created_by_id,updated_by_id,NULL::uuid AS state_id FROM indian_states WHERE sqlc.arg(kind)::text='states'
UNION ALL SELECT id,name,code,normalized_name,is_active,created_at,updated_at,created_by_id,updated_by_id,state_id FROM indian_cities WHERE sqlc.arg(kind)::text='cities') AS records WHERE (NOT sqlc.arg(active_only)::boolean OR is_active) AND (sqlc.arg(kind)::text!='cities' OR sqlc.narg(state_filter)::uuid IS NULL OR state_id=sqlc.narg(state_filter)) ORDER BY name ASC;
-- name: ReferencesLockMutations :exec
SELECT pg_advisory_xact_lock(hashtextextended('epcERP.client_references.administration',0));
-- name: ReferencesRetireCities :exec
UPDATE indian_cities SET is_active=false,updated_by_id=sqlc.arg(actor_id),updated_at=now() WHERE state_id=sqlc.arg(state_id) AND is_active;
-- name: ReferencesInsertClientType :exec
INSERT INTO client_types (id,name,normalized_name,code,is_active,created_by_id,updated_by_id) VALUES (sqlc.arg(id),sqlc.arg(name),sqlc.arg(normalized_name),sqlc.arg(code),sqlc.arg(is_active),sqlc.arg(created_by_id),sqlc.arg(updated_by_id));
-- name: ReferencesUpdateClientType :exec
UPDATE client_types SET name=sqlc.arg(name),normalized_name=sqlc.arg(normalized_name),code=sqlc.arg(code),updated_by_id=sqlc.arg(actor_id),updated_at=now() WHERE id=sqlc.arg(id);
-- name: ReferencesStatusClientType :exec
UPDATE client_types SET is_active=sqlc.arg(active),updated_by_id=sqlc.arg(actor_id),updated_at=now() WHERE id=sqlc.arg(id);
-- name: ReferencesInsertIndustry :exec
INSERT INTO industries (id,name,normalized_name,code,is_active,created_by_id,updated_by_id) VALUES (sqlc.arg(id),sqlc.arg(name),sqlc.arg(normalized_name),sqlc.arg(code),sqlc.arg(is_active),sqlc.arg(created_by_id),sqlc.arg(updated_by_id));
-- name: ReferencesUpdateIndustry :exec
UPDATE industries SET name=sqlc.arg(name),normalized_name=sqlc.arg(normalized_name),code=sqlc.arg(code),updated_by_id=sqlc.arg(actor_id),updated_at=now() WHERE id=sqlc.arg(id);
-- name: ReferencesStatusIndustry :exec
UPDATE industries SET is_active=sqlc.arg(active),updated_by_id=sqlc.arg(actor_id),updated_at=now() WHERE id=sqlc.arg(id);
-- name: ReferencesInsertState :exec
INSERT INTO indian_states (id,name,normalized_name,code,is_active,created_by_id,updated_by_id) VALUES (sqlc.arg(id),sqlc.arg(name),sqlc.arg(normalized_name),sqlc.arg(code),sqlc.arg(is_active),sqlc.arg(created_by_id),sqlc.arg(updated_by_id));
-- name: ReferencesUpdateState :exec
UPDATE indian_states SET name=sqlc.arg(name),normalized_name=sqlc.arg(normalized_name),code=sqlc.arg(code),updated_by_id=sqlc.arg(actor_id),updated_at=now() WHERE id=sqlc.arg(id);
-- name: ReferencesStatusState :exec
UPDATE indian_states SET is_active=sqlc.arg(active),updated_by_id=sqlc.arg(actor_id),updated_at=now() WHERE id=sqlc.arg(id);
-- name: ReferencesInsertCity :exec
INSERT INTO indian_cities (id,name,normalized_name,code,is_active,created_by_id,updated_by_id,state_id) VALUES (sqlc.arg(id),sqlc.arg(name),sqlc.arg(normalized_name),sqlc.arg(code),sqlc.arg(is_active),sqlc.arg(created_by_id),sqlc.arg(updated_by_id),sqlc.arg(state_id));
-- name: ReferencesUpdateCity :exec
UPDATE indian_cities SET name=sqlc.arg(name),normalized_name=sqlc.arg(normalized_name),code=sqlc.arg(code),updated_by_id=sqlc.arg(actor_id),updated_at=now(),state_id=sqlc.arg(state_id) WHERE id=sqlc.arg(id);
-- name: ReferencesStatusCity :exec
UPDATE indian_cities SET is_active=sqlc.arg(active),updated_by_id=sqlc.arg(actor_id),updated_at=now() WHERE id=sqlc.arg(id);
