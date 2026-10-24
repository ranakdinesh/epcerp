package repository

import (
	"context"
	"errors"

	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	projects "github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type QuotationsProviders struct {
	Scope     contracts.WorkspaceAccessGate
	BO        contracts.BOReader
	Documents contracts.IndentDocumentsReader
	Projects  projects.ContextReader
	Clients   masterContracts.ClientDirectory
	Suppliers masterContracts.VendorPurchaseDirectory
}
type quotationsQuoteContext struct{ p QuotationsProviders }

func (c *quotationsQuoteContext) Scope(ctx context.Context, a domain.Principal, p, permission string, write bool) (bool, error) {
	v, e := c.p.Scope.Check(ctx, contracts.WorkspacePrincipal{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, permission, write)
	if errors.Is(e, contracts.WorkspaceErrUnauthorized) {
		return false, domain.QuotationsUnauthorized
	}
	if errors.Is(e, contracts.WorkspaceErrForbidden) {
		return false, domain.QuotationsForbidden
	}
	if errors.Is(e, contracts.WorkspaceErrMissing) {
		return false, domain.QuotationsMissing
	}
	var invalid *contracts.WorkspaceInvalid
	if errors.As(e, &invalid) {
		return false, domain.QuotationsInvalid(invalid.Field, invalid.Message)
	}
	return v.CanWrite, e
}
func (c *quotationsQuoteContext) Source(ctx context.Context, p, b string) (domain.QuotationsScope, error) {
	out := domain.QuotationsScope{Documents: []domain.QuotationsDocument{}}
	source, e := c.p.BO.Get(ctx, p, b)
	if e != nil {
		return out, e
	}
	if source == nil || !source.Active {
		return out, domain.QuotationsMissing
	}
	project, e := c.p.Projects.Get(ctx, p, false)
	if e != nil {
		return out, e
	}
	if project == nil {
		return out, domain.QuotationsMissing
	}
	client, e := c.p.Clients.Get(ctx, project.ClientID)
	if e != nil {
		return out, e
	}
	out.Currency = project.Currency
	out.Available = project.Active && project.PurchaseEnabled && project.Lifecycle != "closed" && project.Lifecycle != "on_hold" && client != nil && client.Active
	candidates, e := c.p.Suppliers.Candidates(ctx)
	if e != nil {
		return out, e
	}
	active := map[string]bool{}
	for _, v := range candidates {
		active[v.ID] = v.Active
	}
	docs, e := c.p.Documents.List(ctx, p, b)
	if e != nil {
		return out, e
	}
	for _, v := range docs {
		if !v.Active {
			continue
		}
		d := domain.QuotationsDocument{ID: v.ID, ProjectID: v.ProjectID, BOID: v.BOID, GroupID: v.GroupID, VendorID: v.VendorID, Version: v.Version, VendorAvailable: active[v.VendorID], Data: domain.QuotationsDocumentData{Terms: v.Data.Terms, Number: v.Data.Number, GroupName: v.Data.GroupName, ScopeRevision: v.Data.ScopeRevision, IsCurrent: v.Data.IsCurrent, SentDate: v.Data.SentDate, Vendor: domain.QuotationsVendor{ID: v.Data.Vendor.ID, Name: v.Data.Vendor.Name, Code: v.Data.Vendor.Code, Active: v.Data.Vendor.Active, Version: v.Data.Vendor.Version, Data: v.Data.Vendor.Data}, Lines: []domain.QuotationsSourceLine{}}}
		for _, l := range v.Data.Lines {
			d.Data.Lines = append(d.Data.Lines, domain.QuotationsSourceLine{ID: l.ID, ItemID: l.ItemID, Quantity: l.Quantity, RequiredDate: l.RequiredDate, Reference: l.Reference, Notes: l.Notes, DemandID: l.DemandID, Snapshot: domain.QuotationsItem{ID: l.Snapshot.ID, SKU: l.Snapshot.SKU, Description: l.Snapshot.Description, UOM: l.Snapshot.UOM, Manufacturer: l.Snapshot.Manufacturer, EngineeringKey: l.Snapshot.EngineeringKey}})
		}
		out.Documents = append(out.Documents, d)
	}
	return out, nil
}
func (c *quotationsQuoteContext) BOChoices(ctx context.Context, p string) ([]domain.QuotationsBOChoice, error) {
	rows, e := c.p.BO.List(ctx, p)
	if e != nil {
		return nil, e
	}
	out := []domain.QuotationsBOChoice{}
	for _, r := range rows {
		if r.Active {
			out = append(out, domain.QuotationsBOChoice{ID: r.ID, Name: r.Name, State: r.State})
		}
	}
	return out, nil
}
