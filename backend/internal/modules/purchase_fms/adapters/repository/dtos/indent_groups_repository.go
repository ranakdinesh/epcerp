package dtos

import (
	"encoding/json"

	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

func IndentGroupsView(row sqlc.PurchaseIndentGroupSet) (domain.IndentGroupsRecord, error) {
	var v domain.IndentGroupsRecord
	if e := json.Unmarshal(row.Data, &v); e != nil {
		return v, e
	}
	v.BOID = row.BoID.String()
	v.ProjectID = row.ProjectID.String()
	v.Version = row.Version
	v.Active = row.IsActive
	v.CreatedAt = row.CreatedAt.Time
	v.UpdatedAt = row.UpdatedAt.Time
	v.CreatedBy = Actor(row.CreatedByID)
	v.UpdatedBy = Actor(row.UpdatedByID)
	return v, nil
}
