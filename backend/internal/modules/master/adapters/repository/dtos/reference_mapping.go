package dtos

import (
	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

func ReferenceFromRow(v db.ReferencesGetReferenceRow) masterDomain.Reference {
	state := ""
	if v.StateID.Valid {
		state = v.StateID.String()
	}
	return masterDomain.Reference{ID: v.ID.String(), Name: v.Name, Code: v.Code, NormalizedName: v.NormalizedName, Active: v.IsActive, StateID: state, CreatedAt: v.CreatedAt.Time.UTC(), UpdatedAt: v.UpdatedAt.Time.UTC(), CreatedBy: Actor(v.CreatedByID), UpdatedBy: Actor(v.UpdatedByID)}
}
