package dtos

import (
	"encoding/json"

	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

func IndentDocumentsView(row sqlc.PurchaseIndentDocument) (domain.IndentDocumentsDocument, error) {
	v := domain.IndentDocumentsDocument{ID: row.ID.String(), ProjectID: row.ProjectID.String(), BOID: row.BoID.String(), GroupID: row.GroupID.String(), VendorID: row.VendorID.String(), Version: row.Version, Active: row.IsActive, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time, CreatedBy: Actor(row.CreatedByID), UpdatedBy: Actor(row.UpdatedByID)}
	e := json.Unmarshal(row.Data, &v.Data)
	return v, e
}
