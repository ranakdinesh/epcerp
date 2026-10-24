package repository

import (
	"context"
	"errors"

	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	projects "github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type IndentDocumentsProviders struct {
	Scope     contracts.WorkspaceAccessGate
	BO        contracts.BOReader
	Groups    contracts.IndentGroupsReader
	Commands  contracts.IndentGroupsCommander
	Suppliers masterContracts.VendorPurchaseDirectory
	Projects  projects.ContextReader
	Clients   masterContracts.ClientDirectory
	Users     auth.AssignmentDirectory
}
type indentDocumentsDocumentContext struct{ p IndentDocumentsProviders }

func (c *indentDocumentsDocumentContext) Scope(ctx context.Context, a domain.Principal, p, permission string, write bool) (bool, error) {
	v, e := c.p.Scope.Check(ctx, contracts.WorkspacePrincipal{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, permission, write)
	if errors.Is(e, contracts.WorkspaceErrUnauthorized) {
		return false, domain.IndentDocumentsUnauthorized
	}
	if errors.Is(e, contracts.WorkspaceErrForbidden) {
		return false, domain.IndentDocumentsForbidden
	}
	if errors.Is(e, contracts.WorkspaceErrMissing) {
		return false, domain.IndentDocumentsMissing
	}
	var invalid *contracts.WorkspaceInvalid
	if errors.As(e, &invalid) {
		return false, domain.IndentDocumentsInvalid(invalid.Field, invalid.Message)
	}
	if e != nil || write {
		return v.CanWrite, e
	}
	project, e := c.p.Projects.Get(ctx, p, false)
	if e != nil {
		return false, e
	}
	if project == nil {
		return false, domain.IndentDocumentsMissing
	}
	client, e := c.p.Clients.Get(ctx, project.ClientID)
	return v.CanWrite && project.Active && project.PurchaseEnabled && project.Lifecycle != "closed" && project.Lifecycle != "on_hold" && client != nil && client.Active, e
}
func (c *indentDocumentsDocumentContext) Revisions(ctx context.Context, p, b string) (map[string]int32, error) {
	source, e := c.p.BO.Get(ctx, p, b)
	if e != nil {
		return nil, e
	}
	if source == nil || !source.Active {
		return nil, domain.IndentDocumentsMissing
	}
	r, e := c.p.Groups.Get(ctx, p, b)
	if e != nil {
		return nil, e
	}
	out := map[string]int32{}
	if r != nil {
		for _, g := range r.Groups {
			out[g.ID] = g.ScopeRevision
		}
	}
	return out, nil
}
func indentDocumentsGroupError(e error) error {
	var v *contracts.IndentGroupsError
	if errors.As(e, &v) {
		return &domain.IndentDocumentsError{Status: v.Status, Field: v.Field, Message: v.Message}
	}
	return e
}
func indentDocumentsGroupActor(a domain.Principal) contracts.IndentGroupsActor {
	return contracts.IndentGroupsActor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}
}
func (c *indentDocumentsDocumentContext) Generate(ctx context.Context, a domain.Principal, p, b string, cmd domain.IndentDocumentsCommand) (domain.IndentDocumentsData, error) {
	r, e := c.p.Groups.Get(ctx, p, b)
	if e != nil {
		return domain.IndentDocumentsData{}, e
	}
	if r == nil {
		return domain.IndentDocumentsData{}, domain.IndentDocumentsInvalid("group_id", "Save item groups and vendors first")
	}
	if r.Version != cmd.GroupVersion {
		return domain.IndentDocumentsData{}, domain.IndentDocumentsStale
	}
	if r.State != "created" {
		input := contracts.IndentGroupsCommand{Action: "create", Version: r.Version, BOVersion: cmd.BOVersion, Reason: "Freeze saved groups for vendor indent generation"}
		for _, g := range r.Groups {
			input.Groups = append(input.Groups, g.IndentGroupsGroupInput)
		}
		w, e := c.p.Commands.Execute(ctx, indentDocumentsGroupActor(a), p, b, input)
		if e != nil {
			return domain.IndentDocumentsData{}, indentDocumentsGroupError(e)
		}
		r = w.Record
	}
	candidates, e := c.p.Suppliers.Candidates(ctx)
	if e != nil {
		return domain.IndentDocumentsData{}, e
	}
	active := map[string]bool{}
	var vendor domain.IndentDocumentsVendor
	for _, v := range candidates {
		active[v.ID] = v.Active
		if v.ID == *cmd.VendorID && v.Active {
			vendor = domain.IndentDocumentsVendor{ID: v.ID, Name: v.Name, Code: v.Code, Active: v.Active, Version: v.Version, Data: v.Data}
		}
	}
	data := domain.IndentDocumentsData{Vendor: vendor, Lines: []domain.IndentDocumentsLine{}}
	found := false
	for _, g := range r.Groups {
		if g.ID != *cmd.GroupID {
			continue
		}
		count := 0
		selected := false
		for _, id := range g.VendorIDs {
			if active[id] {
				count++
			}
			if id == *cmd.VendorID {
				selected = true
			}
		}
		if count < 3 {
			return data, domain.IndentDocumentsInvalid("vendor_id", "Select at least three active vendors before generation")
		}
		if !selected || vendor.ID == "" {
			return data, domain.IndentDocumentsInvalid("vendor_id", "Choose an active vendor selected for this group")
		}
		data.GroupName = g.Name
		data.ScopeRevision = g.ScopeRevision
		for _, line := range r.SourceLines {
			included := false
			for _, id := range g.LineIDs {
				if id == line.ID {
					included = true
				}
			}
			if included {
				data.Lines = append(data.Lines, domain.IndentDocumentsLine{ID: line.ID, ItemID: line.ItemID, Quantity: line.Quantity, RequiredDate: line.RequiredDate, Reference: line.Reference, Notes: line.Notes, DemandID: line.DemandID, Snapshot: domain.IndentDocumentsItem{ID: line.Snapshot.ID, SKU: line.Snapshot.SKU, Description: line.Snapshot.Description, EngineeringKey: line.Snapshot.EngineeringKey, UOM: line.Snapshot.UOM, Manufacturer: line.Snapshot.Manufacturer}})
			}
		}
		found = true
		break
	}
	if !found {
		return data, domain.IndentDocumentsInvalid("group_id", "Select a saved item group")
	}
	project, e := c.p.Projects.Get(ctx, p, false)
	if e != nil {
		return data, e
	}
	if project == nil {
		return data, domain.IndentDocumentsMissing
	}
	user, e := c.p.Users.Get(ctx, a.ID)
	if e != nil {
		return data, e
	}
	if user == nil || !user.Active {
		return data, domain.IndentDocumentsUnauthorized
	}
	data.ProjectName = project.Name
	data.GeneratedByName = user.Name
	return data, nil
}
func (c *indentDocumentsDocumentContext) Dispatch(ctx context.Context, a domain.Principal, d domain.IndentDocumentsDocument, cmd domain.IndentDocumentsCommand) error {
	r, e := c.p.Groups.Get(ctx, d.ProjectID, d.BOID)
	if e != nil {
		return e
	}
	if r == nil {
		return domain.IndentDocumentsMissing
	}
	coverage := []string{}
	for _, l := range d.Data.Lines {
		coverage = append(coverage, l.ID)
	}
	_, e = c.p.Commands.Execute(ctx, indentDocumentsGroupActor(a), d.ProjectID, d.BOID, contracts.IndentGroupsCommand{Action: "invite", Version: r.Version, GroupID: &d.GroupID, VendorID: &d.VendorID, InvitationScopeRevision: &d.Data.ScopeRevision, InvitationLineIDs: coverage, SentDate: cmd.SentDate, Channel: cmd.Channel, DispatchReference: cmd.DispatchReference, Reason: "Record vendor indent dispatch"})
	return indentDocumentsGroupError(e)
}
func (c *indentDocumentsDocumentContext) Recipients(ctx context.Context, p string) (domain.IndentDocumentsRecipients, error) {
	project, e := c.p.Projects.Get(ctx, p, false)
	if e != nil {
		return domain.IndentDocumentsRecipients{}, e
	}
	if project == nil {
		return domain.IndentDocumentsRecipients{}, domain.IndentDocumentsMissing
	}
	users, e := c.p.Users.List(ctx)
	if e != nil {
		return domain.IndentDocumentsRecipients{}, e
	}
	out := domain.IndentDocumentsRecipients{IDs: []string{}}
	for _, u := range users {
		if !u.Active {
			continue
		}
		manager := u.ID == project.ManagerID
		if manager {
			out.ManagerAssigned = true
		}
		if manager || u.Superuser {
			out.IDs = append(out.IDs, u.ID)
		}
	}
	return out, nil
}
