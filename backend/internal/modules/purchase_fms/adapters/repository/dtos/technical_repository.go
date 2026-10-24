package dtos

import (
	"encoding/json"

	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

func TechnicalRecord(v sqlc.PurchaseTechnicalCase) (domain.TechnicalCase, error) {
	var out domain.TechnicalCase
	if e := json.Unmarshal(v.Data, &out); e != nil {
		return out, e
	}
	out.ID = v.ID.String()
	out.ProjectID = v.ProjectID.String()
	out.PackageID = v.PackageID.String()
	out.Version = v.Version
	out.Revision = v.Revision
	out.State = v.State
	out.Active = v.IsActive
	out.CreatedAt = v.CreatedAt.Time.UTC()
	out.UpdatedAt = v.UpdatedAt.Time.UTC()
	out.CreatedBy = v.CreatedByID.String()
	out.UpdatedBy = v.UpdatedByID.String()
	e := json.Unmarshal(v.SourceSnapshot, &out.Source)
	return out, e
}
