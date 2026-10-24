package dtos

import (
	"bytes"

	"encoding/json"

	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

func VendorFromRow(v db.Supplier) (*masterDomain.Vendor, error) {
	data := map[string]any{}
	d := json.NewDecoder(bytes.NewReader(v.Data))
	d.UseNumber()
	if e := DecodeMasterJSON(d, &data); e != nil {
		return nil, e
	}
	return &masterDomain.Vendor{ID: v.ID.String(), Code: v.Code, Name: v.Name, Data: data, Active: v.IsActive, Imported: v.SourceID.Valid, Version: v.Version, CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time, CreatedBy: Actor(v.CreatedByID), UpdatedBy: Actor(v.UpdatedByID), Source: masterDomain.VendorSource{ID: v.SourceID.String(), Version: v.SourceVersion.Int32, CreatedAt: v.SourceCreatedAt.Time, UpdatedAt: v.SourceUpdatedAt.Time}}, nil
}
