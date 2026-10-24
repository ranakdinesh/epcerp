-- name: RecordBudgetDecision :exec
INSERT INTO finance_budget_decisions(id,project_id,source_version,source_revision,amount,currency,source_fingerprint,reason,is_active,created_at,updated_at,created_by_id,updated_by_id)
VALUES(@id,@project_id,@source_version,@source_revision,@amount,@currency,@source_fingerprint,@reason,true,@created_at,@created_at,@actor_id,@actor_id);
-- name: RecordQuotationDecision :exec
INSERT INTO finance_quotation_decisions(id,project_id,bo_id,attempt_id,source_version,submitted_by_id,outcome,amount,currency,source_fingerprint,reason,is_active,created_at,updated_at,created_by_id,updated_by_id)
VALUES(@id,@project_id,@bo_id,@attempt_id,@source_version,@submitted_by_id,@outcome,@amount,@currency,@source_fingerprint,@reason,true,@created_at,@created_at,@actor_id,@actor_id);
-- name: RecordAdvanceDecision :exec
INSERT INTO finance_advance_decisions(id,project_id,package_id,request_id,order_id,order_version,source_version,submitted_by_id,outcome,kind,amount,currency,source_fingerprint,reason,is_active,created_at,updated_at,created_by_id,updated_by_id)
VALUES(@id,@project_id,@package_id,@request_id,@order_id,@order_version,@source_version,@submitted_by_id,@outcome,@kind,@amount,@currency,@source_fingerprint,@reason,true,@created_at,@created_at,@actor_id,@actor_id);
