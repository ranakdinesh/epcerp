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

type InspectionReportsProviders struct {
	Projects   projects.ContextReader
	Purchase   contracts.WorkspaceAccessGate
	Access     access.Gate
	Users      auth.AssignmentDirectory
	Work       contracts.WorkflowSourceReader
	Inspection contracts.InspectionsClearedReader
}
type inspectionReportsSourceContext struct{ p InspectionReportsProviders }

func NewInspectionReportsContext(p InspectionReportsProviders) *inspectionReportsSourceContext {
	return &inspectionReportsSourceContext{p}
}
func inspectionReportsMapped(e error) error {
	var ic *contracts.InspectionsError
	if errors.As(e, &ic) {
		return &domain.InspectionReportsError{Status: ic.Status, Field: ic.Field, Message: ic.Message}
	}
	var a *access.Error
	if errors.As(e, &a) {
		return &domain.InspectionReportsError{Status: a.Status, Field: a.Field, Message: a.Message}
	}
	var w *contracts.WorkflowError
	if errors.As(e, &w) {
		return &domain.InspectionReportsError{Status: w.Status, Field: w.Field, Message: w.Message}
	}
	var p *contracts.WorkspaceInvalid
	if errors.As(e, &p) {
		return domain.InspectionReportsInvalid(p.Field, p.Message)
	}
	if errors.Is(e, contracts.WorkspaceErrForbidden) {
		return domain.InspectionReportsForbidden
	}
	if errors.Is(e, contracts.WorkspaceErrMissing) {
		return domain.InspectionReportsMissing
	}
	if errors.Is(e, contracts.WorkspaceErrUnauthorized) {
		return domain.InspectionReportsUnauthorized
	}
	return e
}
func inspectionReportsCan(a domain.Principal, code string) bool {
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
func (c *inspectionReportsSourceContext) scoped(ctx context.Context, a domain.Principal, p, code string, write bool) (bool, error) {
	v, e := c.p.Purchase.Check(ctx, contracts.WorkspacePrincipal{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	if e == nil {
		return !write || v.CanWrite, nil
	}
	e = inspectionReportsMapped(e)
	var d *domain.InspectionReportsError
	if !errors.As(e, &d) || d.Status != 403 {
		return false, e
	}
	x, e := c.p.Access.Check(ctx, access.Actor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	e = inspectionReportsMapped(e)
	if errors.As(e, &d) && d.Status == 403 {
		return false, nil
	}
	return e == nil && (!write || x.CanWrite), e
}
func (c *inspectionReportsSourceContext) Access(ctx context.Context, a domain.Principal, p, id string) (out domain.InspectionReportsAccess, e error) {

	parent, e := c.p.Projects.Get(ctx, p, true)
	if e != nil {
		return out, e
	}
	if parent == nil {
		return out, domain.InspectionReportsMissing
	}
	u, e := c.p.Users.Get(ctx, a.ID)
	if e != nil {
		return out, e
	}
	if u == nil || !u.Active || u.ID != a.ID {
		return out, domain.InspectionReportsUnauthorized
	}
	a.Superuser = a.Superuser && u.Superuser
	live := func(code string) bool { return inspectionReportsCan(a, code) && u.Allows(code) }
	if !live("projects.purchase_inspection_reports.read") {
		return out, nil
	}
	out.Scoped, e = c.scoped(ctx, a, p, "projects.purchase_inspection_reports.read", false)
	if e != nil {
		return out, e
	}
	out.Read = out.Scoped
	wa := contracts.WorkflowActor{ID: a.ID, Active: true, Superuser: a.Superuser, Permissions: a.Permissions}
	for _, n := range []int{13} {
		allowed, err := c.p.Work.CanWork(ctx, wa, p, id, n)
		if err != nil {
			var d *domain.InspectionReportsError
			err = inspectionReportsMapped(err)
			if !errors.As(err, &d) || d.Status != 422 {
				return out, err
			}
		}
		out.Work = allowed && err == nil
		taskRead, err := c.p.Work.CanReadTask(ctx, wa, p, id, n)
		if err != nil {
			return out, inspectionReportsMapped(err)
		}
		out.Read = out.Read || taskRead
	}
	out.Write = out.Read && live("projects.purchase_inspection_reports.write")

	return out, e
}

func inspectionReportsBlock(out domain.InspectionReportsSource, message string) (domain.InspectionReportsSource, error) {
	out.Available = false
	out.Blockers = append(out.Blockers, message)
	return out, nil
}
func (c *inspectionReportsSourceContext) Source(ctx context.Context, p, id string) (domain.InspectionReportsSource, error) {
	kit, current, e := c.p.Work.Inspect(ctx, p, id)
	if e != nil {
		return domain.InspectionReportsSource{}, inspectionReportsMapped(e)
	}
	out := domain.InspectionReportsSource{InspectionReportsSourceFacts: domain.InspectionReportsSourceFacts{ProjectID: p, PackageID: id}, Available: current.Available && current.Eligible, Blockers: append([]string{}, current.Blockers...)}
	for _, step := range kit.Steps {
		if step.Number == 13 && step.State == "blocked" {
			out.Blockers = append(out.Blockers, "IR / IRN has an unresolved blocker")
		}
	}
	actual, e := c.p.Inspection.Cleared(ctx, p, id)
	if e != nil {
		e = inspectionReportsMapped(e)
		var d *domain.InspectionReportsError
		if errors.As(e, &d) && (d.Status == 404 || d.Status == 409 || d.Status == 422) {
			return inspectionReportsBlock(out, d.Message)
		}
		return out, e
	}
	if actual == nil {
		return inspectionReportsBlock(out, "Current material inspection must be cleared before recording IR / IRN")
	}
	if actual.ProjectID != p || actual.PackageID != id {
		return domain.InspectionReportsSource{}, domain.InspectionReportsInvalid("source", "Inspection belongs to another project/package")
	}
	source := actual.Source
	out.InspectionReportsSourceFacts = domain.InspectionReportsSourceFacts{ProjectID: p, PackageID: id, InspectionID: actual.ID, InspectionVersion: actual.Version, InspectionRevision: actual.Revision, ActualDate: actual.Inspection.ActualDate, OrderID: source.OrderID, OrderVersion: source.OrderVersion, Number: source.Number, SupplierName: source.SupplierName, Description: source.Description, Quantity: source.Quantity, UOM: source.UOM, QAPReference: source.QAPReference}
	return out, nil
}
