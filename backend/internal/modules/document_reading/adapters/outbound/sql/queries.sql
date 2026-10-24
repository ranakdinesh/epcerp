-- name: SubmitReading :one
INSERT INTO document_readings(id,module,entity_id,filename,format,size_bytes,sha256,original,created_by_id,updated_by_id)VALUES(@id,@module,@entity_id,@filename,@format,@size_bytes,@sha256,@original,@actor_id,@actor_id) ON CONFLICT(module,entity_id,sha256) DO NOTHING RETURNING *;
-- name: GetReading :one
SELECT * FROM document_readings WHERE id= @id AND module= @module AND entity_id= @entity_id;
-- name: FindReading :one
SELECT * FROM document_readings WHERE module= @module AND entity_id= @entity_id AND sha256= @sha256;
-- name: ListReadings :many
SELECT id,module,entity_id,filename,format,size_bytes,sha256,status,version,attempts,error_message,segment_count,warnings,created_at,updated_at,created_by_id FROM document_readings WHERE module= @module AND entity_id= @entity_id ORDER BY created_at,id;
-- name: RetryReading :one
UPDATE document_readings SET status='queued',error_message='',claim_id=NULL,lease_until=NULL,version=version+1,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE id= @id AND status='failed' AND attempts<3 AND version= @expected_version RETURNING *;
-- name: ClaimReading :one
WITH candidate AS(SELECT id FROM document_readings WHERE status='queued' OR (status='processing' AND lease_until<clock_timestamp()) ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1)
UPDATE document_readings d SET status=CASE WHEN attempts>=3 THEN 'failed' ELSE 'processing' END, attempts=least(attempts+1,3),version=version+1,claim_id= @claim_id,lease_until=clock_timestamp()+interval '20 minutes',error_message=CASE WHEN attempts>=3 THEN 'Processing was interrupted three times; upload a revised document after review' ELSE '' END,updated_at=clock_timestamp(),updated_by_id=NULL FROM candidate c WHERE d.id=c.id RETURNING d.*;
-- name: FinishReading :one
UPDATE document_readings SET status= @status,error_message= @error_message,segment_count= @segment_count,warnings= @warnings,lease_until=NULL,claim_id=NULL,version=version+1,updated_at=clock_timestamp(),updated_by_id=NULL WHERE id= @id AND status='processing' AND claim_id= @claim_id AND lease_until>clock_timestamp() RETURNING *;
-- name: InsertReadingResult :exec
INSERT INTO document_reading_results(document_id,result,result_sha256,reader_version)VALUES(@document_id,@result,@result_sha256,@reader_version);
-- name: GetReadingResult :one
SELECT result,result_sha256 FROM document_reading_results WHERE document_id= @document_id;
-- name: InsertReadingEvent :exec
INSERT INTO document_reading_events(id,document_id,action,version,actor_id)VALUES(@id,@document_id,@action,@version,@actor_id);
-- name: ReadingHistory :many
SELECT * FROM document_reading_events WHERE document_id= @document_id ORDER BY created_at DESC,id;
