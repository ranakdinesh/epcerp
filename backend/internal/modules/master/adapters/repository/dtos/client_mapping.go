package dtos

import (
	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

func ClientFromRow(r db.ClientsGetClientRow) masterDomain.Client {
	c := masterDomain.Client{ID: r.ID.String(), Code: r.ClientCode, NormalizedName: r.NormalizedLegalName, Active: r.IsActive, CreatedAt: r.CreatedAt.Time.UTC(), UpdatedAt: r.UpdatedAt.Time.UTC(), CreatedBy: Actor(r.CreatedByID), UpdatedBy: Actor(r.UpdatedByID), Contacts: []masterDomain.ClientContact{},
		LegalName: r.LegalName, DisplayName: r.DisplayName, ParentOrganization: r.ParentOrganization, ClientType: r.ClientType, Industry: r.Industry, Gstin: r.Gstin, Pan: r.Pan, Cin: r.Cin, Website: r.Website, AddressLine1: r.AddressLine1, AddressLine2: r.AddressLine2, City: r.City, State: r.State, PostalCode: r.PostalCode, Country: r.Country}
	return c
}
