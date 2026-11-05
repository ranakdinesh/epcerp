-- name: TemplatesLockTemplates :exec
SELECT pg_advisory_xact_lock(91827145);
-- name: TemplatesGetTemplate :one
SELECT * FROM configuration_templates WHERE subcategory_id= @subcategory_id ORDER BY revision DESC LIMIT 1;
-- name: TemplatesListFields :many
SELECT * FROM template_fields WHERE template_id= @template_id ORDER BY sort_order;
-- name: TemplatesListOptions :many
SELECT o.* FROM template_options o JOIN template_fields f ON f.id=o.field_id WHERE f.template_id= @template_id ORDER BY o.sort_order;
-- name: TemplatesInsertTemplate :exec
INSERT INTO configuration_templates(id,subcategory_id,template_name,model_required,revision,is_active,legacy_definition,source_fingerprint,created_by_id,updated_by_id)
VALUES(@id,@subcategory_id,@template_name,@model_required,@revision,@is_active,@legacy_definition,@source_fingerprint,@actor_id,@actor_id);
-- name: TemplatesInsertField :exec
INSERT INTO template_fields(id,template_id,field_code,field_name,input_type,mandatory,allow_na,use_in_description,description_order,description_prefix,description_suffix,use_in_duplicate_check,sort_order,is_active,created_by_id,updated_by_id)
VALUES(@id,@template_id,@field_code,@field_name,@input_type,@mandatory,@allow_na,@use_in_description,@description_order,@description_prefix,@description_suffix,@use_in_duplicate_check,@sort_order,@is_active,@actor_id,@actor_id);
-- name: TemplatesInsertOption :exec
INSERT INTO template_options(id,field_id,option_code,option_label,sort_order,is_active,created_by_id,updated_by_id)
VALUES(@id,@field_id,@option_code,@option_label,@sort_order,@is_active,@actor_id,@actor_id);

-- name: TemplatesInsertPublication :exec
INSERT INTO template_publications(template_id,previous_template_id,reason,before_definition,after_definition,created_by_id,updated_by_id)
VALUES(@template_id,@previous_template_id,@reason,@before_definition,@after_definition,@actor_id,@actor_id);

-- name: TemplatesGetTemplateByID :one
SELECT * FROM configuration_templates WHERE id= @id;
-- name: TemplatesListLatestTemplates :many
SELECT DISTINCT ON(subcategory_id) * FROM configuration_templates ORDER BY subcategory_id,revision DESC;
