package dtos

import (
	"encoding/json"

	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

func HoldsView(v sqlc.PurchaseHold) (domain.HoldsRecord, error) {
	out := domain.HoldsRecord{ID: v.ID.String(), ProjectID: v.ProjectID.String(), Active: v.IsActive, Version: v.Version, CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time, CreatedBy: Actor(v.CreatedByID), UpdatedBy: Actor(v.UpdatedByID)}
	e := json.Unmarshal(v.Data, &out.Data)
	return out, e
}
