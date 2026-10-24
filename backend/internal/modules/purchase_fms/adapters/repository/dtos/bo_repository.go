package dtos

import (
	"github.com/jackc/pgx/v5/pgtype"
)

func BONullableID(s *string) pgtype.UUID {
	if s == nil {
		return pgtype.UUID{}
	}
	return UUID(*s)
}
func BONullableText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}
