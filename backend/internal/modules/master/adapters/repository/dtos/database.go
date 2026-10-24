package dtos

import "github.com/jackc/pgx/v5/pgtype"

// UUID translates optional string identifiers to PostgreSQL UUID values.
func UUID(s string) pgtype.UUID { var v pgtype.UUID; _ = v.Scan(s); return v }

// Actor translates nullable PostgreSQL actor identifiers to domain pointers.
func Actor(v pgtype.UUID) *string {
	if !v.Valid {
		return nil
	}
	s := v.String()
	return &s
}
