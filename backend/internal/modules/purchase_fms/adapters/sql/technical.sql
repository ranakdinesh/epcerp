-- name: TechnicalGetCase :one
SELECT * FROM purchase_technical_cases WHERE project_id= @project_id AND package_id= @package_id FOR UPDATE;
-- name: TechnicalInsertCase :one
INSERT INTO purchase_technical_cases(id,project_id,package_id,source_case_id,version,revision,state,source_snapshot,data,is_active,created_at,updated_at,created_by_id,updated_by_id)VALUES(@id,@project_id,@package_id,@source_case_id,1,1,@state,@source_snapshot,@data,true,@at,@at,@actor_id,@actor_id)RETURNING *;
-- name: TechnicalUpdateCase :one
UPDATE purchase_technical_cases SET version=version+1,revision= @revision,state= @state,data= @data,updated_at= @at,updated_by_id= @actor_id WHERE id= @id AND version= @expected_version RETURNING *;
-- name: TechnicalEvent :exec
INSERT INTO purchase_technical_events(id,subject_id,action,reason,before_snapshot,after_snapshot,created_at,created_by_id)VALUES(@id,@case_id,@action,@reason,@before_snapshot,@after_snapshot,@at,@actor_id);
-- name: TechnicalHistory :many
SELECT * FROM purchase_technical_events WHERE subject_id= @case_id ORDER BY created_at DESC,id LIMIT 200;
-- name: TechnicalFiles :many
SELECT id,case_id,purpose,name,media_type,byte_size,sha256,is_active FROM purchase_technical_files WHERE case_id= @case_id ORDER BY created_at,id;
-- name: TechnicalEvidence :one
SELECT * FROM purchase_technical_files WHERE case_id= @case_id AND id= @id AND is_active;
-- name: TechnicalStoreEvidence :exec
INSERT INTO purchase_technical_files(id,case_id,purpose,name,media_type,byte_size,sha256,ciphertext,object_bucket,object_key,is_active,created_at,updated_at,created_by_id,updated_by_id)VALUES(@id,@case_id,@purpose,@name,@media_type,@byte_size,@sha256,@ciphertext,@object_bucket,@object_key,true,@at,@at,@actor_id,@actor_id);
-- name: TechnicalEvidenceEvent :exec
INSERT INTO purchase_technical_file_events(id,case_id,file_id,action,reason,created_at,created_by_id)VALUES(@id,@case_id,@file_id,@action,@reason,@at,@actor_id);
-- name: TechnicalCaseProject :one
SELECT project_id FROM purchase_technical_cases WHERE id= @id;
