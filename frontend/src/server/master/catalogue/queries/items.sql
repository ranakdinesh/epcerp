-- name: ItemsLockItems :exec
SELECT pg_advisory_xact_lock(91827146);
-- name: ItemsListItems :many
SELECT * FROM catalogue_items ORDER BY created_at DESC,id;
-- name: ItemsGetItem :one
SELECT * FROM catalogue_items WHERE id= @id;
-- name: ItemsFindItem :one
SELECT * FROM catalogue_items WHERE fingerprint= @fingerprint;
-- name: ItemsLegacySkuCandidates :many
SELECT * FROM catalogue_items WHERE snapshot->'configuration'->>'category_code'= @category_code::text AND serial IS NOT NULL;
-- name: ItemsCreateItem :one
INSERT INTO catalogue_items(id,configuration_id,manufacturer_id,category_id,model,specifications,snapshot,fingerprint,engineering_key,description,sku,status,is_active,created_by_id,updated_by_id)
VALUES(@id,@configuration_id,@manufacturer_id,@category_id,@model,@specifications,@snapshot,@fingerprint,@engineering_key,@description,@sku,'approved',true,@actor_id,@actor_id)
ON CONFLICT (sku) DO NOTHING RETURNING *;
-- name: ItemsChangeItem :one
UPDATE catalogue_items SET status= @status,is_active= @is_active,version=version+1,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE id= @id AND version= @expected_version RETURNING *;
-- name: ItemsRecordItemEvent :exec
INSERT INTO catalogue_item_events(id,subject_id,action,reason,before_snapshot,after_snapshot,created_by_id) VALUES(@id,@subject_id,@action,@reason,@before_snapshot,@after_snapshot,@actor_id);
-- name: ItemsItemHistory :many
SELECT * FROM catalogue_item_events WHERE subject_id= @subject_id ORDER BY created_at DESC,id;

-- name: ItemsIdentityCandidates :many
SELECT * FROM catalogue_items WHERE specifications= @specifications::jsonb AND snapshot->'configuration'->>'family_code'= @family::text AND snapshot->'configuration'->>'uom'= @uom::text ORDER BY created_at,id;
