-- name: ListHolidays :many
SELECT * FROM work_calendar_holidays WHERE extract(year FROM holiday_date)::integer=sqlc.arg(year)::integer ORDER BY holiday_date;
-- name: HolidayDates :many
SELECT holiday_date FROM work_calendar_holidays WHERE is_active;
-- name: CreateHoliday :one
INSERT INTO work_calendar_holidays(id,name,holiday_date,is_active,created_by_id,updated_by_id) VALUES(sqlc.arg(id),sqlc.arg(name),sqlc.arg(holiday_date),sqlc.arg(is_active),sqlc.arg(actor),sqlc.arg(actor)) RETURNING *;
-- name: UpdateHoliday :one
UPDATE work_calendar_holidays SET name=sqlc.arg(name),holiday_date=sqlc.arg(holiday_date),is_active=sqlc.arg(is_active),version=version+1,updated_by_id=sqlc.arg(actor),updated_at=clock_timestamp() WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) RETURNING *;
-- name: HolidayHistory :exec
INSERT INTO work_calendar_changes(id,holiday_id,version,reason,snapshot,created_by_id) VALUES(sqlc.arg(id),sqlc.arg(holiday_id),sqlc.arg(version),sqlc.arg(reason),sqlc.arg(snapshot),sqlc.arg(actor));
