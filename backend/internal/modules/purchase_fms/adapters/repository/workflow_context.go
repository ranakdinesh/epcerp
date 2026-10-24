package repository

import (
	"context"
	"errors"

	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	access "github.com/ranakdinesh/epcERP/internal/modules/project_access/contracts"
	projects "github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type WorkflowProviders struct {
	Projects    projects.ContextReader
	Purchase    contracts.WorkspaceAccessGate
	Access      access.Gate
	Users       auth.AssignmentDirectory
	Indents     contracts.IndentsReader
	Preparation contracts.IndentsPreparationReader
}
type workflowSourceContext struct{ p WorkflowProviders }

func workflowMapped(e error) error {
	var a *access.Error
	if errors.As(e, &a) {
		return &domain.WorkflowError{Status: a.Status, Field: a.Field, Message: a.Message}
	}
	var i *contracts.IndentsError
	if errors.As(e, &i) {
		return &domain.WorkflowError{Status: i.Status, Field: i.Field, Message: i.Message}
	}
	var p *contracts.WorkspaceInvalid
	if errors.As(e, &p) {
		return domain.WorkflowInvalid(p.Field, p.Message)
	}
	if errors.Is(e, contracts.WorkspaceErrForbidden) {
		return domain.WorkflowForbidden
	}
	if errors.Is(e, contracts.WorkspaceErrMissing) {
		return domain.WorkflowMissing
	}
	if errors.Is(e, contracts.WorkspaceErrUnauthorized) {
		return domain.WorkflowUnauthorized
	}
	return e
}
func (c *workflowSourceContext) Actions(ctx context.Context, a domain.Principal, p string) (out domain.WorkflowAccess, e error) {
	parent, e := c.p.Projects.Get(ctx, p, true)
	if e != nil {
		return out, e
	}
	if parent == nil {
		return out, domain.WorkflowMissing
	}
	u, e := c.p.Users.Get(ctx, a.ID)
	if e != nil {
		return out, e
	}
	if u == nil || !u.Active {
		return out, domain.WorkflowUnauthorized
	}
	live := func(code string) bool { return a.Can(code) && u.Allows(code) }
	scoped := func(code string, write bool) (bool, error) {
		if !live(code) {
			return false, nil
		}
		v, e := c.p.Purchase.Check(ctx, contracts.WorkspacePrincipal{ID: a.ID, Active: true, Superuser: a.Superuser && u.Superuser, Permissions: a.Permissions}, p, code, write)
		if e == nil {
			return !write || v.CanWrite, nil
		}
		e = workflowMapped(e)
		var d *domain.WorkflowError
		if !errors.As(e, &d) || d.Status != 403 {
			return false, e
		}
		x, e := c.p.Access.Check(ctx, access.Actor{ID: a.ID, Active: true, Superuser: a.Superuser && u.Superuser, Permissions: a.Permissions}, p, code, write)
		e = workflowMapped(e)
		if errors.As(e, &d) && d.Status == 403 {
			return false, nil
		}
		return e == nil && (!write || x.CanWrite), e
	}
	view, e := scoped("projects.purchase_workflow.view", false)
	if e != nil {
		return out, e
	}
	read, e := scoped("projects.purchase_workflow.read", false)
	if e != nil {
		return out, e
	}
	write, e := scoped("projects.purchase_workflow.write", true)
	if e != nil {
		var d *domain.WorkflowError
		if !errors.As(e, &d) || d.Status != 422 {
			return out, e
		}
		write = false
	}
	return domain.WorkflowAccess{Recovery: a.Superuser && u.Superuser, Read: view && read, Write: view && read && write, Execute: live("projects.purchase_workflow.execute"), Resolve: live("projects.purchase_workflow.resolve")}, nil
}
func (c *workflowSourceContext) Members(ctx context.Context, _ string) ([]domain.WorkflowMember, error) {
	users, e := c.p.Users.List(ctx)
	if e != nil {
		return nil, e
	}
	out := []domain.WorkflowMember{}
	for _, option := range users {
		// Re-read each live candidate on this transaction. The Auth directory
		// holds a share lock until commit, serializing assignment with deactivation.
		u, e := c.p.Users.Get(ctx, option.ID)
		if e != nil {
			return nil, e
		}
		if u == nil {
			continue
		}
		if !u.Active {
			continue
		}
		execute, resolve := u.Allows("projects.purchase_workflow.execute"), u.Allows("projects.purchase_workflow.resolve")
		if execute || resolve {
			out = append(out, domain.WorkflowMember{ID: u.ID, Label: u.Name, Execute: execute, Resolve: resolve})
		}
	}
	return out, nil
}
func (c *workflowSourceContext) Candidates(ctx context.Context, p string) ([]domain.WorkflowCandidate, error) {
	rows, e := c.p.Indents.List(ctx, p)
	if e != nil {
		return nil, workflowMapped(e)
	}
	out := []domain.WorkflowCandidate{}
	for _, r := range rows {
		if !r.Active {
			continue
		}
		for _, line := range r.Data.Released {
			out = append(out, domain.WorkflowCandidate{IndentID: r.ID, IndentNumber: r.Data.Number, DemandID: line.DemandID, Description: line.Source.Item.Description, Quantity: line.Quantity})
		}
	}
	return out, nil
}
func (c *workflowSourceContext) Source(ctx context.Context, p, indent, demand string) (out domain.WorkflowSource, e error) {
	parent, e := c.p.Projects.Get(ctx, p, true)
	if e != nil {
		return out, e
	}
	if parent == nil {
		return out, domain.WorkflowMissing
	}
	r, e := c.p.Indents.Get(ctx, p, indent)
	if e != nil {
		return out, workflowMapped(e)
	}
	if r == nil {
		return out, domain.WorkflowMissing
	}
	current, e := c.p.Preparation.Line(ctx, p, demand)
	if e != nil {
		return out, workflowMapped(e)
	}
	out = domain.WorkflowSource{ProjectName: parent.Name, Site: parent.SiteName, DemandID: demand, IndentNumber: r.Data.Number, Available: parent.Active && parent.PurchaseEnabled && parent.Lifecycle != "on_hold" && parent.Lifecycle != "closed", Eligible: current.Active && current.Policy.Status == "eligible" && r.Active, Blockers: append([]string{}, current.Policy.Reasons...)}
	for _, h := range current.Holds.Holds {
		out.Blockers = append(out.Blockers, h.Reason)
	}
	if !r.Active {
		out.Blockers = append(out.Blockers, "Source indent is withdrawn")
	}
	if !current.Active {
		out.Blockers = append(out.Blockers, "Current requirement, item or client is inactive")
	}
	found := false
	for _, line := range r.Data.Released {
		if line.DemandID != demand {
			continue
		}
		found = true
		out.Description = line.Source.Item.Description
		out.Quantity = line.Quantity
		out.UOM = line.Source.Item.UOM
		out.RequiredDate = line.Source.RequiredDate
		out.FullKit = line.FullKit
		out.ReleaseAt = line.At
		if current.DemandVersion != line.Source.DemandVersion {
			out.Blockers = append(out.Blockers, "Project requirement changed since indent release; source review required")
		}
	}
	if !found {
		return out, domain.WorkflowInvalid("demand_id", "Only a released indent line can create a purchase package")
	}
	return out, nil
}
