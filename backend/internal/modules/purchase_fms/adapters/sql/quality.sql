-- name: QualityGetCase :one
SELECT * FROM purchase_quality_cases WHERE project_id=sqlc.arg(project_id) AND package_id=sqlc.arg(package_id) FOR UPDATE;
-- name: QualityInsertCase :one
INSERT INTO purchase_quality_cases(id,project_id,package_id,report_id,report_version,revision,version,source_snapshot,data,created_at,updated_at,created_by_id,updated_by_id)
VALUES(sqlc.arg(id),sqlc.arg(project_id),sqlc.arg(package_id),sqlc.arg(report_id),sqlc.arg(report_version),sqlc.arg(revision),1,sqlc.arg(source_snapshot),sqlc.arg(data),sqlc.arg(at),sqlc.arg(at),sqlc.arg(actor_id),sqlc.arg(actor_id)) RETURNING *;
-- name: QualityUpdateCase :one
UPDATE purchase_quality_cases SET version=version+1,source_snapshot=sqlc.arg(source_snapshot),report_id=sqlc.arg(report_id),report_version=sqlc.arg(report_version),revision=sqlc.arg(revision),data=sqlc.arg(data),updated_at=sqlc.arg(at),updated_by_id=sqlc.arg(actor_id)
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) AND is_active RETURNING *;
-- name: QualityEvent :exec
INSERT INTO purchase_quality_events(id,subject_id,action,certificate_code,reason,before_snapshot,after_snapshot,created_at,created_by_id)
VALUES(sqlc.arg(id),sqlc.arg(case_id),sqlc.arg(action),sqlc.arg(certificate_code),sqlc.arg(reason),sqlc.arg(before_snapshot),sqlc.arg(after_snapshot),sqlc.arg(at),sqlc.arg(actor_id));
-- name: QualityHistory :many
SELECT * FROM purchase_quality_events WHERE subject_id=$1 ORDER BY created_at DESC,id DESC;

-- name: QualitySaveCertificate :exec
INSERT INTO purchase_quality_certificates(case_id,code,title,cert_number,issued_by,valid_until,file_name,verified,verified_at,verified_by_id,data,is_active,created_at,updated_at,created_by_id,updated_by_id)
SELECT sqlc.arg(case_id)::uuid,d->>'code',d->>'title',d->>'cert_number',d->>'issued_by',nullif(d->>'valid_until','')::date,d->>'file_name',(d->>'verified')::boolean,(d->>'verified_at')::timestamptz,nullif(d->>'verified_by_id','')::uuid,d,(d->>'is_active')::boolean,(d->>'created_at')::timestamptz,(d->>'updated_at')::timestamptz,(d->>'created_by_id')::uuid,(d->>'updated_by_id')::uuid
FROM (SELECT sqlc.arg(data)::jsonb AS d) input
ON CONFLICT(case_id,code) DO UPDATE SET title=excluded.title,cert_number=excluded.cert_number,issued_by=excluded.issued_by,valid_until=excluded.valid_until,file_name=excluded.file_name,verified=excluded.verified,verified_at=excluded.verified_at,verified_by_id=excluded.verified_by_id,data=excluded.data,is_active=excluded.is_active,updated_at=excluded.updated_at,updated_by_id=excluded.updated_by_id;
