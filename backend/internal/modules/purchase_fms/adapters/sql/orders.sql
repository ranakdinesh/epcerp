-- name: OrdersGetOrder :one
SELECT * FROM purchase_orders WHERE project_id= @project_id AND package_id= @package_id FOR UPDATE;
-- name: OrdersInsertOrder :one
INSERT INTO purchase_orders(id,project_id,package_id,sourcing_id,technical_id,version,state,source_snapshot,data,is_active,created_at,updated_at,created_by_id,updated_by_id)VALUES(@id,@project_id,@package_id,@sourcing_id,@technical_id,1,'draft',@source_snapshot,@data,true,@at,@at,@actor_id,@actor_id)RETURNING *;
-- name: OrdersUpdateOrder :one
UPDATE purchase_orders SET version=version+1,state= @state,number= @number,source_snapshot= @source_snapshot,data= @data,updated_at= @at,updated_by_id= @actor_id WHERE id= @id AND version= @expected_version AND state='draft' RETURNING *;
-- name: OrdersEvent :exec
INSERT INTO purchase_order_events(id,subject_id,action,reason,before_snapshot,after_snapshot,created_at,created_by_id)VALUES(@id,@order_id,@action,@reason,@before_snapshot,@after_snapshot,@at,@actor_id);
-- name: OrdersHistory :many
SELECT * FROM purchase_order_events WHERE subject_id= @order_id ORDER BY created_at DESC,id LIMIT 200;
-- name: OrdersNextNumber :one
INSERT INTO purchase_order_sequences(fiscal_year,last_value)VALUES(@fiscal_year,1)ON CONFLICT(fiscal_year)DO UPDATE SET last_value=purchase_order_sequences.last_value+1 RETURNING last_value;

-- name: OrdersDocumentEvent :exec
INSERT INTO purchase_order_document_events(id,order_id,order_version,created_at,created_by_id)VALUES(@id,@order_id,@order_version,@at,@actor_id);

-- name: OrdersClientFile :one
SELECT * FROM purchase_order_client_files WHERE order_id= @order_id AND id= @id AND is_active;
-- name: OrdersStoreClientFile :exec
INSERT INTO purchase_order_client_files(id,order_id,draft_revision,name,media_type,byte_size,sha256,object_bucket,object_key,is_active,created_at,updated_at,created_by_id,updated_by_id)
VALUES(@id,@order_id,@draft_revision,@name,@media_type,@byte_size,@sha256,@object_bucket,@object_key,true,@at,@at,@actor_id,@actor_id);
-- name: OrdersClientFileEvent :exec
INSERT INTO purchase_order_client_file_events(id,file_id,order_id,action,created_at,created_by_id)VALUES(@id,@file_id,@order_id,@action,@at,@actor_id);
-- name: OrdersGetByID :one
SELECT * FROM purchase_orders WHERE id= @id;
