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

type DispatchProviders struct {
	Projects  projects.ContextReader
	Purchase  contracts.WorkspaceAccessGate
	Access    access.Gate
	Users     auth.AssignmentDirectory
	Work      contracts.WorkflowSourceReader
	Clearance contracts.ClientClearanceClearedReader
}
type dispatchSourceContext struct{ p DispatchProviders }

func NewDispatchContext(p DispatchProviders) *dispatchSourceContext { return &dispatchSourceContext{p} }
func dispatchMapped(e error) error {
	var ic *contracts.ClientClearanceError
	if errors.As(e, &ic) {
		return &domain.DispatchError{Status: ic.Status, Field: ic.Field, Message: ic.Message}
	}
	var a *access.Error
	if errors.As(e, &a) {
		return &domain.DispatchError{Status: a.Status, Field: a.Field, Message: a.Message}
	}
	var w *contracts.WorkflowError
	if errors.As(e, &w) {
		return &domain.DispatchError{Status: w.Status, Field: w.Field, Message: w.Message}
	}
	var p *contracts.WorkspaceInvalid
	if errors.As(e, &p) {
		return domain.DispatchInvalid(p.Field, p.Message)
	}
	if errors.Is(e, contracts.WorkspaceErrForbidden) {
		return domain.DispatchForbidden
	}
	if errors.Is(e, contracts.WorkspaceErrMissing) {
		return domain.DispatchMissing
	}
	if errors.Is(e, contracts.WorkspaceErrUnauthorized) {
		return domain.DispatchUnauthorized
	}
	return e
}
func dispatchCan(a domain.Principal, code string) bool {
	if !a.Active {
		return false
	}
	if a.Superuser {
		return true
	}
	for _, p := range a.Permissions {
		if p == code {
			return true
		}
	}
	return false
}
func (c *dispatchSourceContext) scoped(ctx context.Context, a domain.Principal, p, code string, write bool) (bool, error) {
	v, e := c.p.Purchase.Check(ctx, contracts.WorkspacePrincipal{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	if e == nil {
		return !write || v.CanWrite, nil
	}
	e = dispatchMapped(e)
	var d *domain.DispatchError
	if !errors.As(e, &d) || d.Status != 403 {
		return false, e
	}
	x, e := c.p.Access.Check(ctx, access.Actor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	e = dispatchMapped(e)
	if errors.As(e, &d) && d.Status == 403 {
		return false, nil
	}
	return e == nil && (!write || x.CanWrite), e
}
func (c *dispatchSourceContext) Access(ctx context.Context, a domain.Principal, p, id string) (out domain.DispatchAccess, e error) {

	parent, e := c.p.Projects.Get(ctx, p, true)
	if e != nil {
		return out, e
	}
	if parent == nil {
		return out, domain.DispatchMissing
	}
	u, e := c.p.Users.Get(ctx, a.ID)
	if e != nil {
		return out, e
	}
	if u == nil || !u.Active || u.ID != a.ID {
		return out, domain.DispatchUnauthorized
	}
	a.Superuser = a.Superuser && u.Superuser
	live := func(code string) bool { return dispatchCan(a, code) && u.Allows(code) }
	if !live("projects.purchase_dispatch.read") {
		return out, nil
	}
	out.Scoped, e = c.scoped(ctx, a, p, "projects.purchase_dispatch.read", false)
	if e != nil {
		return out, e
	}
	out.Read = out.Scoped
	wa := contracts.WorkflowActor{ID: a.ID, Active: true, Superuser: a.Superuser, Permissions: a.Permissions}
	for _, n := range []int{16} {
		allowed, err := c.p.Work.CanWork(ctx, wa, p, id, n)
		if err != nil {
			var d *domain.DispatchError
			err = dispatchMapped(err)
			if !errors.As(err, &d) || d.Status != 422 {
				return out, err
			}
		}
		out.Work = allowed && err == nil
		taskRead, err := c.p.Work.CanReadTask(ctx, wa, p, id, n)
		if err != nil {
			return out, dispatchMapped(err)
		}
		out.Read = out.Read || taskRead
	}
	out.Write = out.Read && live("projects.purchase_dispatch.write")

	return out, e
}

func dispatchBlock(out domain.DispatchSource, message string) (domain.DispatchSource, error) {
	out.Available = false
	out.Blockers = append(out.Blockers, message)
	return out, nil
}
func (c *dispatchSourceContext) Source(ctx context.Context, p, id string) (domain.DispatchSource, error) {
	kit, current, e := c.p.Work.Inspect(ctx, p, id)
	if e != nil {
		return domain.DispatchSource{}, dispatchMapped(e)
	}
	out := domain.DispatchSource{DispatchSourceFacts: domain.DispatchSourceFacts{ProjectID: p, PackageID: id}, Available: current.Available && current.Eligible, Blockers: append([]string{}, current.Blockers...)}
	for _, step := range kit.Steps {
		if step.Number == 16 && step.State == "blocked" {
			out.Blockers = append(out.Blockers, "Material dispatch has an unresolved blocker")
		}
	}
	actual, e := c.p.Clearance.Cleared(ctx, p, id)
	if e != nil {
		e = dispatchMapped(e)
		var d *domain.DispatchError
		if errors.As(e, &d) && (d.Status == 404 || d.Status == 409 || d.Status == 422) {
			return dispatchBlock(out, d.Message)
		}
		return out, e
	}
	if actual == nil {
		return dispatchBlock(out, "Current client MDCC must be cleared before recording material dispatch")
	}
	if actual.ProjectID != p || actual.PackageID != id {
		return domain.DispatchSource{}, domain.DispatchInvalid("source", "Report belongs to another project/package")
	}
	source := actual.Source
	out.DispatchSourceFacts = domain.DispatchSourceFacts{ProjectID: p, PackageID: id, ClearanceID: actual.ID, ClearanceVersion: actual.Version, ClearanceRevision: actual.Revision, Reference: actual.Clearance.Reference, ClearanceDate: actual.Clearance.Date, QualityID: source.QualityID, QualityVersion: source.QualityVersion, QualityRevision: source.QualityRevision, ReportID: source.ReportID, ReportVersion: source.ReportVersion, ReportRevision: source.ReportRevision, IRNumber: source.IRNumber, IRNNumber: source.IRNNumber, IssueDate: source.IssueDate, OrderID: source.OrderID, OrderVersion: source.OrderVersion, Number: source.Number, SupplierName: source.SupplierName, Description: source.Description, Quantity: source.Quantity, UOM: source.UOM, InspectionID: source.InspectionID, InspectionVersion: source.InspectionVersion}
	return out, nil
}
