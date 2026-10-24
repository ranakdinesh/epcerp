package dtos

import (
	"encoding/json"

	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

func InspectionsRecord(v sqlc.PurchaseInspection) (domain.InspectionsRecord, error) {
	var out domain.InspectionsRecord
	if e := json.Unmarshal(v.Data, &out); e != nil {
		return out, e
	}
	out.ID = v.ID.String()
	out.ProjectID = v.ProjectID.String()
	out.PackageID = v.PackageID.String()
	out.Version = v.Version
	out.Revision = v.Revision
	out.Inspection.Status = v.Status
	out.Active = v.IsActive
	out.CreatedAt = v.CreatedAt.Time.UTC()
	out.UpdatedAt = v.UpdatedAt.Time.UTC()
	out.CreatedBy = v.CreatedByID.String()
	out.UpdatedBy = v.UpdatedByID.String()
	e := json.Unmarshal(v.SourceSnapshot, &out.Source)
	return out, e
}
