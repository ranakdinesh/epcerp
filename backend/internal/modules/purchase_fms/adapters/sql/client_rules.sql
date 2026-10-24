-- name: ClientRulesLockRules :exec
SELECT pg_advisory_xact_lock(91827148);
-- name: ClientRulesListRules :many
SELECT * FROM client_purchase_rules WHERE client_id= @client_id ORDER BY created_at,id;
-- name: ClientRulesGetRule :one
SELECT * FROM client_purchase_rules WHERE id= @id AND client_id= @client_id;
-- name: ClientRulesInsertRule :one
INSERT INTO client_purchase_rules(id,client_id,project_id,data,is_active,version,created_by_id,updated_by_id) VALUES(@id,@client_id,@project_id,@data,@is_active,1,@actor_id,@actor_id) RETURNING *;
-- name: ClientRulesUpdateRule :one
UPDATE client_purchase_rules SET project_id= @project_id,data= @data,is_active= @is_active,version=version+1,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE id= @id AND client_id= @client_id AND version= @expected_version RETURNING *;
-- name: ClientRulesInsertRuleEvent :exec
INSERT INTO client_purchase_rule_events(id,subject_id,action,reason,before_snapshot,after_snapshot,created_by_id)VALUES(@id,@subject_id,@action,@reason,@before_snapshot,@after_snapshot,@actor_id);
-- name: ClientRulesRuleHistory :many
SELECT e.* FROM client_purchase_rule_events e JOIN client_purchase_rules r ON r.id=e.subject_id WHERE e.subject_id= @id AND r.client_id= @client_id ORDER BY e.created_at DESC,e.id;
