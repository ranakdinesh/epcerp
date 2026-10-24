package dtos

import (
	"encoding/json"

	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

func WorkflowRecord(v sqlc.PurchaseWorkPackage) (domain.WorkflowPackage, error) {
	out := domain.WorkflowPackage{ID: v.ID.String(), ProjectID: v.ProjectID.String(), IndentID: v.IndentID.String(), DemandID: v.DemandID.String(), Code: v.Code, Description: v.Description, Version: v.Version, Active: v.IsActive, CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time, CreatedBy: Actor(v.CreatedByID), UpdatedBy: Actor(v.UpdatedByID)}
	if e := json.Unmarshal(v.SourceSnapshot, &out.Source); e != nil {
		return out, e
	}
	e := json.Unmarshal(v.Steps, &out.Steps)
	return out, e
}
