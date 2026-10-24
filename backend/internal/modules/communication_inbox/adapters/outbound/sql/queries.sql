-- name: NoticeSource :one
SELECT * FROM business_events WHERE id=$1 FOR SHARE;
-- name: NoticeBySource :one
SELECT * FROM communication_notices WHERE source_event_id=$1 AND notice_kind='director';
-- name: CreateNotice :one
INSERT INTO communication_notices(id,source_event_id,recipient_id,director_designation_id,director_version,decision_id,project_id,source_version,decision_action,created_at,updated_at,created_by_id,updated_by_id,notice_kind,title,link,source_action_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10,$11,$11,sqlc.arg(notice_kind),sqlc.arg(title),sqlc.arg(link),sqlc.narg(source_action_id)) ON CONFLICT(source_event_id,recipient_id) DO NOTHING RETURNING *;
-- name: OwnNotice :one
SELECT * FROM communication_notices WHERE id=$1 AND recipient_id=$2 FOR UPDATE;
-- name: OwnNotices :many
SELECT * FROM communication_notices WHERE recipient_id=sqlc.arg(recipient_id) AND (sqlc.arg(status)::text='' OR (sqlc.arg(status)::text='unread' AND read_at IS NULL) OR (sqlc.arg(status)::text='read' AND read_at IS NOT NULL AND acknowledged_at IS NULL) OR (sqlc.arg(status)::text='acknowledged' AND acknowledged_at IS NOT NULL)) ORDER BY created_at DESC,id DESC LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);
-- name: OwnNoticeCount :one
SELECT count(*) FROM communication_notices WHERE recipient_id=sqlc.arg(recipient_id) AND (sqlc.arg(status)::text='' OR (sqlc.arg(status)::text='unread' AND read_at IS NULL) OR (sqlc.arg(status)::text='read' AND read_at IS NOT NULL AND acknowledged_at IS NULL) OR (sqlc.arg(status)::text='acknowledged' AND acknowledged_at IS NOT NULL));
-- name: SaveReceipt :one
UPDATE communication_notices SET version=version+1,read_at=sqlc.narg(read_at),acknowledged_at=sqlc.narg(acknowledged_at),updated_at=sqlc.arg(updated_at),updated_by_id=sqlc.arg(recipient_id) WHERE id=sqlc.arg(id) AND recipient_id=sqlc.arg(recipient_id) AND version=sqlc.arg(expected_version) RETURNING *;
-- name: CreateNoticeEvent :exec
INSERT INTO communication_notice_events(id,notice_id,action,notice_version,before_snapshot,after_snapshot,created_at,created_by_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8);
-- name: OwnNoticeHistory :many
SELECT e.* FROM communication_notice_events e JOIN communication_notices n ON n.id=e.notice_id WHERE n.id=$1 AND n.recipient_id=$2 ORDER BY e.notice_version DESC LIMIT 100;

-- name: NoticeByRecipientSource :one
SELECT * FROM communication_notices WHERE source_event_id=$1 AND recipient_id=$2;
