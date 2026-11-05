-- name: CategoriesLockImport :exec
SELECT pg_advisory_xact_lock(91827144);

-- name: CategoriesListCategories :many
SELECT * FROM categories ORDER BY code;

-- name: CategoriesInsertCategory :exec
INSERT INTO categories (id,code,name,is_active,created_by_id,updated_by_id,legacy_source,source_fingerprint,unit_id)
VALUES (@id,@code,@name,@is_active,@actor_id,@actor_id,@legacy_source,@source_fingerprint,@unit_id);

-- name: CategoriesListUnits :many
SELECT * FROM catalogue_units ORDER BY code;

-- name: CategoriesInsertUnit :exec
INSERT INTO catalogue_units (id,code,name,is_active,created_by_id,updated_by_id,legacy_source,source_fingerprint)
VALUES (@id,@code,@name,@is_active,@actor_id,@actor_id,@legacy_source,@source_fingerprint);

-- name: CategoriesListSubcategories :many
SELECT * FROM category_subcategories ORDER BY code;

-- name: CategoriesInsertSubcategory :exec
INSERT INTO category_subcategories (id,code,name,is_active,created_by_id,updated_by_id,legacy_source,source_fingerprint,category_id,unit_id,configuration)
VALUES (@id,@code,@name,@is_active,@actor_id,@actor_id,@legacy_source,@source_fingerprint,@category_id,@unit_id,@configuration);

-- name: CategoriesInsertCreationHistory :exec
INSERT INTO category_catalogue_changes(id,kind,subject_id,action,reason,before_snapshot,after_snapshot,created_by_id)
VALUES(@id,@kind,@subject_id,@action,@reason,@before_snapshot,@after_snapshot,@actor_id);

-- name: CategoriesUpdateCategory :execrows
UPDATE categories SET code= @code,name= @name,version=version+1,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE id= @id AND version= @expected_version;

-- name: CategoriesUpdateSubcategory :execrows
UPDATE category_subcategories SET code= @code,name= @name,version=version+1,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE id= @id AND version= @expected_version;

-- name: CategoriesUpdateUnit :execrows
UPDATE catalogue_units SET code= @code,name= @name,version=version+1,updated_at=clock_timestamp(),updated_by_id= @actor_id WHERE id= @id AND version= @expected_version;

-- name: CategoriesCountBrowse :one
WITH records AS (SELECT id, code, name, is_active, version, NULL::uuid AS category_id, unit_id FROM categories WHERE @kind::text='category'
 UNION ALL SELECT id,code,name,is_active,version,category_id,unit_id FROM category_subcategories WHERE @kind::text='subcategory' AND category_id=sqlc.narg(category_id)::uuid
 UNION ALL SELECT id,code,name,is_active,version,NULL::uuid,NULL::uuid FROM catalogue_units WHERE @kind::text='unit' ) SELECT count(*) FROM records r WHERE (@search::text='' OR strpos(lower(r.code||' '||r.name), lower(@search::text))>0)
 AND (@status::text='' OR r.is_active=(@status::text='Active'));

-- name: CategoriesBrowseRecords :many
WITH records AS (SELECT id, code, name, is_active, version, NULL::uuid AS category_id, unit_id FROM categories WHERE @kind::text='category'
 UNION ALL SELECT id,code,name,is_active,version,category_id,unit_id FROM category_subcategories WHERE @kind::text='subcategory' AND category_id=sqlc.narg(category_id)::uuid
 UNION ALL SELECT id,code,name,is_active,version,NULL::uuid,NULL::uuid FROM catalogue_units WHERE @kind::text='unit' ), page AS (
 SELECT r.* FROM records r WHERE (@search::text='' OR strpos(lower(r.code||' '||r.name), lower(@search::text))>0)
 AND (@status::text='' OR r.is_active=(@status::text='Active'))
 ORDER BY
 CASE WHEN @sort::text='name' AND @direction::text='asc' THEN lower(r.name) END ASC,
 CASE WHEN @sort::text='name' AND @direction::text='desc' THEN lower(r.name) END DESC,
 CASE WHEN @sort::text='code' AND @direction::text='asc' THEN lower(r.code) END ASC,
 CASE WHEN @sort::text='code' AND @direction::text='desc' THEN lower(r.code) END DESC,
 CASE WHEN @sort::text='unit' AND @direction::text='asc' THEN (SELECT code FROM catalogue_units WHERE id=r.unit_id) END ASC,
 CASE WHEN @sort::text='unit' AND @direction::text='desc' THEN (SELECT code FROM catalogue_units WHERE id=r.unit_id) END DESC,
 CASE WHEN @sort::text='status' AND @direction::text='asc' THEN r.is_active END ASC,
 CASE WHEN @sort::text='status' AND @direction::text='desc' THEN r.is_active END DESC,
 CASE WHEN @sort::text='types' AND @direction::text='asc' THEN (SELECT count(*) FROM category_subcategories WHERE category_id=r.id) END ASC,
 CASE WHEN @sort::text='types' AND @direction::text='desc' THEN (SELECT count(*) FROM category_subcategories WHERE category_id=r.id) END DESC,
 r.id ASC LIMIT @page_size::integer OFFSET @page_offset::integer
)
SELECT p.*, coalesce(u.code,'')::text AS unit_code,coalesce(u.name,'')::text AS unit_name,
 CASE WHEN @kind::text='category' THEN (SELECT count(*) FROM category_subcategories WHERE category_id=p.id) ELSE 0 END::bigint AS child_count
FROM page p LEFT JOIN catalogue_units u ON u.id=p.unit_id ORDER BY
 CASE WHEN @sort::text='name' AND @direction::text='asc' THEN lower(p.name) END ASC,
 CASE WHEN @sort::text='name' AND @direction::text='desc' THEN lower(p.name) END DESC,
 CASE WHEN @sort::text='code' AND @direction::text='asc' THEN lower(p.code) END ASC,
 CASE WHEN @sort::text='code' AND @direction::text='desc' THEN lower(p.code) END DESC,
 CASE WHEN @sort::text='unit' AND @direction::text='asc' THEN (SELECT code FROM catalogue_units WHERE id=p.unit_id) END ASC,
 CASE WHEN @sort::text='unit' AND @direction::text='desc' THEN (SELECT code FROM catalogue_units WHERE id=p.unit_id) END DESC,
 CASE WHEN @sort::text='status' AND @direction::text='asc' THEN p.is_active END ASC,
 CASE WHEN @sort::text='status' AND @direction::text='desc' THEN p.is_active END DESC,
 CASE WHEN @sort::text='types' AND @direction::text='asc' THEN (SELECT count(*) FROM category_subcategories WHERE category_id=p.id) END ASC,
 CASE WHEN @sort::text='types' AND @direction::text='desc' THEN (SELECT count(*) FROM category_subcategories WHERE category_id=p.id) END DESC,
 p.id ASC;

-- name: CategoriesCatalogueDetail :one
WITH records AS (
 SELECT id,code,name,is_active,version,NULL::uuid AS category_id,unit_id,NULL::jsonb AS configuration FROM categories WHERE @kind::text='category' AND id= @identity::uuid
 UNION ALL SELECT id,code,name,is_active,version,category_id,unit_id,configuration FROM category_subcategories WHERE @kind::text='subcategory' AND id= @identity::uuid
 UNION ALL SELECT id,code,name,is_active,version,NULL::uuid,NULL::uuid,NULL::jsonb FROM catalogue_units WHERE @kind::text='unit' AND id= @identity::uuid
)
SELECT r.*,coalesce(u.code,'')::text AS unit_code,coalesce(u.name,'')::text AS unit_name,
 CASE WHEN @kind::text='category' THEN (SELECT count(*) FROM category_subcategories WHERE category_id=r.id) ELSE 0 END::bigint AS child_count
FROM records r LEFT JOIN catalogue_units u ON u.id=r.unit_id;

-- name: CategoriesFindSubcategoryByCode :one
SELECT s.id, (s.is_active AND c.is_active AND u.is_active)::boolean AS active FROM category_subcategories s JOIN categories c ON c.id=s.category_id JOIN catalogue_units u ON u.id=s.unit_id WHERE s.code= @code;

-- name: CategoriesFindEligibility :many
WITH choices AS (
 SELECT c.code,c.name,'category'::text AS kind,''::text AS parent_name,''::text AS parent_code,(c.is_active AND u.is_active)::boolean AS active FROM categories c JOIN catalogue_units u ON u.id=c.unit_id
 UNION ALL SELECT s.code,s.name,'subcategory'::text,c.name::text AS parent_name,c.code::text AS parent_code,(s.is_active AND c.is_active AND u.is_active)::boolean AS active FROM category_subcategories s JOIN categories c ON c.id=s.category_id JOIN catalogue_units u ON u.id=s.unit_id
)
SELECT * FROM choices WHERE code=ANY(@codes::text[]);

-- name: CategoriesBrowseEligibility :many
WITH choices AS (
 SELECT c.code,c.name,'category'::text AS kind,''::text AS parent_name,''::text AS parent_code,(c.is_active AND u.is_active)::boolean AS active FROM categories c JOIN catalogue_units u ON u.id=c.unit_id
 UNION ALL SELECT s.code,s.name,'subcategory'::text,c.name::text AS parent_name,c.code::text AS parent_code,(s.is_active AND c.is_active AND u.is_active)::boolean AS active FROM category_subcategories s JOIN categories c ON c.id=s.category_id JOIN catalogue_units u ON u.id=s.unit_id
)
SELECT * FROM choices WHERE active AND (@kind::text='' OR kind= @kind::text) AND (@category_code::text='' OR parent_code= @category_code::text) AND (strpos(lower(name),lower(@search::text))>0 OR strpos(lower(code),lower(@search::text))>0 OR strpos(lower(parent_name),lower(@search::text))>0) ORDER BY name,kind,code LIMIT 50 OFFSET @skip::integer;

-- name: CategoriesCountEligibility :one
WITH choices AS (
 SELECT c.code,c.name,'category'::text AS kind,''::text AS parent_name,''::text AS parent_code,(c.is_active AND u.is_active)::boolean AS active FROM categories c JOIN catalogue_units u ON u.id=c.unit_id
 UNION ALL SELECT s.code,s.name,'subcategory'::text,c.name::text AS parent_name,c.code::text AS parent_code,(s.is_active AND c.is_active AND u.is_active)::boolean AS active FROM category_subcategories s JOIN categories c ON c.id=s.category_id JOIN catalogue_units u ON u.id=s.unit_id
)
SELECT count(*) FROM choices WHERE active AND (@kind::text='' OR kind= @kind::text) AND (@category_code::text='' OR parent_code= @category_code::text) AND (strpos(lower(name),lower(@search::text))>0 OR strpos(lower(code),lower(@search::text))>0 OR strpos(lower(parent_name),lower(@search::text))>0);

-- name: CategoriesItemContexts :many
SELECT s.id,s.code,s.name,c.id AS category_id,c.code AS category_code,c.name AS category_name,u.code AS unit_code,COALESCE(NULLIF(s.configuration->>'family_code',''),s.code)::text AS family_code,(s.is_active AND c.is_active AND u.is_active)::boolean AS active
FROM category_subcategories s JOIN categories c ON c.id=s.category_id JOIN catalogue_units u ON u.id=s.unit_id ORDER BY c.code,s.code;

-- name: CategoriesGetItemContext :one
SELECT s.id,s.code,s.name,c.id AS category_id,c.code AS category_code,c.name AS category_name,u.code AS unit_code,COALESCE(NULLIF(s.configuration->>'family_code',''),s.code)::text AS family_code,(s.is_active AND c.is_active AND u.is_active)::boolean AS active
FROM category_subcategories s JOIN categories c ON c.id=s.category_id JOIN catalogue_units u ON u.id=s.unit_id WHERE s.id= @id FOR SHARE OF s,c,u;
