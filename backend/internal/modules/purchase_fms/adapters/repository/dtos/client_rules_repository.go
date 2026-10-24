package dtos

import (
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"

	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

func UUID(s string) pgtype.UUID { var v pgtype.UUID; _ = v.Scan(s); return v }
func Actor(v pgtype.UUID) *string {
	if !v.Valid {
		return nil
	}
	s := v.String()
	return &s
}

func ClientRulesView(v sqlc.ClientPurchaseRule) (domain.ClientRulesRecord, error) {
	out := domain.ClientRulesRecord{ID: v.ID.String(), ClientID: v.ClientID.String(), Active: v.IsActive, Version: v.Version, CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time, CreatedBy: Actor(v.CreatedByID), UpdatedBy: Actor(v.UpdatedByID)}
	e := json.Unmarshal(v.Data, &out.Data)
	return out, e
}
