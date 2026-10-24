package dtos

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func UUID(value string) pgtype.UUID            { var result pgtype.UUID; _ = result.Scan(value); return result }
func Stamp(value time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: value, Valid: true} }
func Nullable(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *value, Valid: true}
}
func Boolean(value *bool) pgtype.Bool {
	if value == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *value, Valid: true}
}
