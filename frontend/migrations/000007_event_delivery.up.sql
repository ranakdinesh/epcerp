-- Infrastructure only: source modules retain business rules and authorization.
BEGIN;
CREATE TABLE business_events (
 id UUID PRIMARY KEY,
 source_module VARCHAR(64) NOT NULL CHECK(source_module ~ '^[a-z][a-z0-9_.]{0,63}$'),
 entity_type VARCHAR(64) NOT NULL CHECK(entity_type ~ '^[a-z][a-z0-9_.]{0,63}$'),
 entity_id UUID NOT NULL,
 entity_version BIGINT NOT NULL CHECK(entity_version>0),
 delivery_count INTEGER NOT NULL CHECK(delivery_count BETWEEN 0 AND 32),
 action VARCHAR(64) NOT NULL CHECK(action ~ '^[a-z][a-z0-9_.]{0,63}$'),
 actor_id UUID REFERENCES users(id),
 actor_kind VARCHAR(16) NOT NULL CHECK(actor_kind IN ('user','system')),
 CHECK((actor_kind='user' AND actor_id IS NOT NULL) OR (actor_kind='system' AND actor_id IS NULL)),
 correlation_id UUID NOT NULL,
 causation_id UUID,
 scope_kind VARCHAR(16) NOT NULL CHECK(scope_kind IN ('platform','project','tender')),
 scope_id UUID,
 visibility VARCHAR(16) NOT NULL CHECK(visibility='restricted'),
 occurred_at TIMESTAMPTZ NOT NULL,
 recorded_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 CHECK((scope_kind='platform' AND scope_id IS NULL) OR (scope_kind IN ('project','tender') AND scope_id IS NOT NULL)),
 UNIQUE(source_module,entity_type,entity_id,entity_version,action)
);
CREATE INDEX business_events_scope_idx ON business_events(scope_kind,scope_id,recorded_at,id);
CREATE INDEX business_events_actor_idx ON business_events(actor_id) WHERE actor_id IS NOT NULL;
CREATE TABLE event_deliveries (
 id UUID PRIMARY KEY,
 event_id UUID NOT NULL REFERENCES business_events(id),
 consumer VARCHAR(64) NOT NULL CHECK(consumer ~ '^[a-z][a-z0-9_.]{0,63}$'),
 state VARCHAR(16) NOT NULL DEFAULT 'queued' CHECK(state IN ('queued','leased','delivered','dead')),
 attempts INTEGER NOT NULL DEFAULT 0 CHECK(attempts>=0),
 max_attempts INTEGER NOT NULL CHECK(max_attempts BETWEEN 1 AND 20),
 available_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 lease_token UUID,
 lease_until TIMESTAMPTZ,
 delivered_at TIMESTAMPTZ,
 failure_code VARCHAR(32) CHECK(failure_code IN ('handler_failed','lease_expired')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(event_id,consumer),
 CHECK(attempts<=max_attempts),
 CHECK((state='leased' AND lease_token IS NOT NULL AND lease_until IS NOT NULL) OR (state<>'leased' AND lease_token IS NULL AND lease_until IS NULL)),
 CHECK((state='delivered' AND delivered_at IS NOT NULL) OR (state<>'delivered' AND delivered_at IS NULL))
);
CREATE INDEX event_deliveries_queue_idx ON event_deliveries(consumer,available_at,id) WHERE state='queued';
CREATE INDEX event_deliveries_lease_idx ON event_deliveries(consumer,lease_until,id) WHERE state='leased';
CREATE TABLE event_delivery_attempts (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 delivery_id UUID NOT NULL REFERENCES event_deliveries(id),
 attempt INTEGER NOT NULL CHECK(attempt BETWEEN 1 AND 20),
 lease_token UUID NOT NULL,
 kind VARCHAR(16) NOT NULL CHECK(kind IN ('claimed','expired','retry','dead','delivered')),
 failure_code VARCHAR(32) CHECK(failure_code IN ('handler_failed','lease_expired')),
 occurred_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(delivery_id,attempt,kind)
);
CREATE FUNCTION epcerp_reject_event_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'Immutable event evidence cannot be changed' USING ERRCODE='55000';
END;
$$;
CREATE TRIGGER business_events_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON business_events FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
CREATE TRIGGER event_delivery_attempts_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON event_delivery_attempts FOR EACH STATEMENT EXECUTE FUNCTION epcerp_reject_event_mutation();
COMMIT;
