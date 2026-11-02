-- name: ListTSCFormats :many
SELECT f.id,f.name,f.description,f.is_active,f.version,f.updated_at,(SELECT count(*) FROM tender.tsc_format_questions q WHERE q.format_id=f.id) AS question_count
FROM tender.tsc_formats f WHERE (NOT sqlc.arg(active_only)::boolean OR f.is_active) ORDER BY f.name_key LIMIT 500;
-- name: GetTSCFormat :one
SELECT * FROM tender.tsc_formats WHERE id=$1 FOR UPDATE;
-- name: ListTSCFormatQuestions :many
SELECT id,section,printed_number,question,manual_review,legacy_row FROM tender.tsc_format_questions WHERE format_id=$1 ORDER BY position;
-- name: CreateTSCFormat :exec
INSERT INTO tender.tsc_formats(id,name,description,is_active,created_by_id,updated_by_id) VALUES($1,$2,$3,$4,$5,$5);
-- name: UpdateTSCFormat :execrows
UPDATE tender.tsc_formats SET name=$2,description=$3,is_active=$4,version=version+1,updated_by_id=$5,updated_at=clock_timestamp() WHERE id=$1 AND version=$6;
-- name: ClearTSCFormatQuestions :exec
DELETE FROM tender.tsc_format_questions WHERE format_id=$1;
-- name: InsertTSCFormatQuestion :exec
INSERT INTO tender.tsc_format_questions(format_id,id,position,section,printed_number,question,manual_review,legacy_row) VALUES($1,$2,$3,$4,$5,$6,$7,$8);
-- name: InsertTSCFormatEvent :exec
INSERT INTO tender.tsc_format_events(id,format_id,version,actor_id,before_snapshot,after_snapshot) VALUES($1,$2,$3,$4,$5,$6);
