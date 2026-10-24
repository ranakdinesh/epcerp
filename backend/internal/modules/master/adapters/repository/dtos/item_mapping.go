package dtos

import (
	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

func ItemFromRow(v db.CatalogueItem) (masterDomain.Item, error) {
	out := masterDomain.Item{ID: v.ID.String(), ConfigurationID: v.ConfigurationID.String(), ManufacturerID: v.ManufacturerID.String(), CategoryID: v.CategoryID.String(), Model: v.Model, Fingerprint: v.Fingerprint, EngineeringKey: v.EngineeringKey, Description: v.Description, SKU: v.Sku, Serial: v.Serial, Status: v.Status, Active: v.IsActive, Version: v.Version, CreatedAt: v.CreatedAt.Time.UTC(), UpdatedAt: v.UpdatedAt.Time.UTC(), CreatedBy: Actor(v.CreatedByID), UpdatedBy: Actor(v.UpdatedByID)}
	if e := UnmarshalMasterJSON(v.Specifications, &out.Specifications); e != nil {
		return out, e
	}
	e := UnmarshalMasterJSON(v.Snapshot, &out.Snapshot)
	return out, e
}
