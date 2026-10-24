BEGIN;
DROP TABLE event_delivery_attempts;
DROP TABLE event_deliveries;
DROP TABLE business_events;
DROP FUNCTION epcerp_reject_event_mutation();
COMMIT;
