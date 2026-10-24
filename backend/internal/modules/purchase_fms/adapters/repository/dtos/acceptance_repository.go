package dtos

import (
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"

	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

func Stamp(t domain.AcceptanceTimestamp) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func AcceptanceRecord(v sqlc.PurchaseAcceptance) (domain.AcceptanceRecord, error) {
	var out domain.AcceptanceRecord
	if e := json.Unmarshal(v.Data, &out); e != nil {
		return out, e
	}
	out.ID = v.ID.String()
	out.ProjectID = v.ProjectID.String()
	out.PackageID = v.PackageID.String()
	out.Version = v.Version
	out.Response.Status = v.Status
	out.Active = v.IsActive
	out.CreatedAt = v.CreatedAt.Time.UTC()
	out.UpdatedAt = v.UpdatedAt.Time.UTC()
	out.CreatedBy = v.CreatedByID.String()
	out.UpdatedBy = v.UpdatedByID.String()
	e := json.Unmarshal(v.OrderSnapshot, &out.Order)
	return out, e
}
