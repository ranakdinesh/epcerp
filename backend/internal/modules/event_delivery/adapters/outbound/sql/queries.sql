-- name: InsertEvent :execrows
INSERT INTO business_events(id,source_module,entity_type,entity_id,entity_version,delivery_count,action,actor_id,actor_kind,correlation_id,causation_id,scope_kind,scope_id,visibility,occurred_at)
VALUES(sqlc.arg(id),sqlc.arg(source_module),sqlc.arg(entity_type),sqlc.arg(entity_id),sqlc.arg(entity_version),sqlc.arg(delivery_count),sqlc.arg(action),sqlc.narg(actor_id),sqlc.arg(actor_kind),sqlc.arg(correlation_id),sqlc.narg(causation_id),sqlc.arg(scope_kind),sqlc.narg(scope_id),sqlc.arg(visibility),sqlc.arg(occurred_at)) ON CONFLICT(id) DO NOTHING;
-- name: GetEvent :one
SELECT * FROM business_events WHERE id=$1;
-- name: InsertDelivery :exec
INSERT INTO event_deliveries(id,event_id,consumer,max_attempts) VALUES($1,$2,$3,$4) ON CONFLICT(event_id,consumer) DO NOTHING;
-- name: DeliveryPolicy :one
SELECT max_attempts FROM event_deliveries WHERE event_id=$1 AND consumer=$2;

-- name: ClaimDelivery :one
WITH candidate AS (
 SELECT q.id,q.state,q.attempts,q.lease_token FROM event_deliveries q
 WHERE q.consumer=sqlc.arg(consumer) AND q.attempts<q.max_attempts AND
 ((q.state='queued' AND q.available_at<=clock_timestamp()) OR (q.state='leased' AND q.lease_until<=clock_timestamp()))
 ORDER BY q.available_at,q.id LIMIT 1 FOR UPDATE SKIP LOCKED
), claimed AS (
 UPDATE event_deliveries d SET state='leased',attempts=d.attempts+1,
 lease_token=sqlc.arg(lease_token),lease_until=clock_timestamp()+make_interval(secs=>sqlc.arg(lease_seconds)::integer),updated_at=clock_timestamp()
 FROM candidate c WHERE d.id=c.id RETURNING d.*
), expired_history AS (
 INSERT INTO event_delivery_attempts(delivery_id,attempt,lease_token,kind,failure_code)
 SELECT id,attempts,lease_token,'expired','lease_expired' FROM candidate WHERE state='leased'
), claimed_history AS (
 INSERT INTO event_delivery_attempts(delivery_id,attempt,lease_token,kind)
 SELECT id,attempts,lease_token,'claimed' FROM claimed
)
SELECT e.*,d.id AS delivery_id,d.consumer,d.lease_token,d.attempts FROM claimed d JOIN business_events e ON e.id=d.event_id;

-- name: CompleteDelivery :one
WITH completed AS (
 UPDATE event_deliveries q SET state='delivered',lease_token=NULL,lease_until=NULL,delivered_at=clock_timestamp(),failure_code=NULL,updated_at=clock_timestamp()
 WHERE q.id=sqlc.arg(id) AND q.state='leased' AND q.lease_token=sqlc.arg(lease_token) AND q.lease_until>clock_timestamp()
 RETURNING q.id,q.attempts
), history AS (
 INSERT INTO event_delivery_attempts(delivery_id,attempt,lease_token,kind)
 SELECT id,attempts,sqlc.arg(lease_token),'delivered' FROM completed
)
SELECT count(*) FROM completed;

-- name: FailDelivery :one
WITH failed AS (
 UPDATE event_deliveries q SET state=CASE WHEN q.attempts>=q.max_attempts THEN 'dead' ELSE 'queued' END,
 lease_token=NULL,lease_until=NULL,failure_code='handler_failed',
 available_at=clock_timestamp()+make_interval(secs=>sqlc.arg(retry_seconds)::integer),updated_at=clock_timestamp()
 WHERE q.id=sqlc.arg(id) AND q.state='leased' AND q.lease_token=sqlc.arg(lease_token) AND q.lease_until>clock_timestamp()
 RETURNING q.id,q.attempts,q.state
), history AS (
 INSERT INTO event_delivery_attempts(delivery_id,attempt,lease_token,kind,failure_code)
 SELECT id,attempts,sqlc.arg(lease_token),CASE WHEN state='dead' THEN 'dead' ELSE 'retry' END,'handler_failed' FROM failed
)
SELECT count(*) FROM failed;

-- name: ExpireExhausted :one
WITH candidate AS (
 SELECT q.id,q.lease_token FROM event_deliveries q WHERE q.consumer=$1 AND q.state='leased' AND q.attempts>=q.max_attempts AND q.lease_until<=clock_timestamp()
 ORDER BY q.lease_until,q.id LIMIT 1000 FOR UPDATE SKIP LOCKED
), expired AS (
 UPDATE event_deliveries d SET state='dead',lease_token=NULL,lease_until=NULL,failure_code='lease_expired',updated_at=clock_timestamp()
 FROM candidate c WHERE d.id=c.id RETURNING d.id,d.attempts,c.lease_token
), history AS (
 INSERT INTO event_delivery_attempts(delivery_id,attempt,lease_token,kind,failure_code)
 SELECT id,attempts,lease_token,'dead','lease_expired' FROM expired
)
SELECT count(*) FROM expired;
