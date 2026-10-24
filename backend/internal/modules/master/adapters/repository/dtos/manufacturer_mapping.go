package dtos

import (
	"bytes"

	"encoding/json"

	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

func ManufacturerDataFromJSON(raw []byte) (map[string]any, error) {
	var v map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	e := DecodeMasterJSON(decoder, &v)
	return v, e
}
func ManufacturerFromRow(row db.Manufacturer) (masterDomain.Manufacturer, error) {
	data, e := ManufacturerDataFromJSON(row.Data)
	if e != nil {
		return masterDomain.Manufacturer{}, e
	}
	return masterDomain.Manufacturer{ID: row.ID.String(), Code: row.Code, Name: row.Name, Data: data, Active: row.IsActive, Version: row.Version, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(), CreatedBy: Actor(row.CreatedByID), UpdatedBy: Actor(row.UpdatedByID)}, nil
}
