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

type QualityProviders struct {
	Projects projects.ContextReader
	Purchase contracts.WorkspaceAccessGate
	Access   access.Gate
	Users    auth.AssignmentDirectory
	Work     contracts.WorkflowSourceReader
	Reports  contracts.InspectionReportsIssuedReader
}
type qualitySourceContext struct{ p QualityProviders }

func NewQualityContext(p QualityProviders) *qualitySourceContext { return &qualitySourceContext{p} }
func qualityMapped(e error) error {
	var ic *contracts.InspectionReportsError
	if errors.As(e, &ic) {
		return &domain.QualityError{Status: ic.Status, Field: ic.Field, Message: ic.Message}
	}
	var a *access.Error
	if errors.As(e, &a) {
		return &domain.QualityError{Status: a.Status, Field: a.Field, Message: a.Message}
	}
	var w *contracts.WorkflowError
	if errors.As(e, &w) {
		return &domain.QualityError{Status: w.Status, Field: w.Field, Message: w.Message}
	}
	var p *contracts.WorkspaceInvalid
	if errors.As(e, &p) {
		return domain.QualityInvalid(p.Field, p.Message)
	}
	if errors.Is(e, contracts.WorkspaceErrForbidden) {
		return domain.QualityForbidden
	}
	if errors.Is(e, contracts.WorkspaceErrMissing) {
		return domain.QualityMissing
	}
	if errors.Is(e, contracts.WorkspaceErrUnauthorized) {
		return domain.QualityUnauthorized
	}
	return e
}
func qualityCan(a domain.Principal, code string) bool {
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
func (c *qualitySourceContext) scoped(ctx context.Context, a domain.Principal, p, code string, write bool) (bool, error) {
	v, e := c.p.Purchase.Check(ctx, contracts.WorkspacePrincipal{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	if e == nil {
		return !write || v.CanWrite, nil
	}
	e = qualityMapped(e)
	var d *domain.QualityError
	if !errors.As(e, &d) || d.Status != 403 {
		return false, e
	}
	x, e := c.p.Access.Check(ctx, access.Actor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	e = qualityMapped(e)
	if errors.As(e, &d) && d.Status == 403 {
		return false, nil
	}
	return e == nil && (!write || x.CanWrite), e
}
func (c *qualitySourceContext) Access(ctx context.Context, a domain.Principal, p, id string) (out domain.QualityAccess, e error) {

	parent, e := c.p.Projects.Get(ctx, p, true)
	if e != nil {
		return out, e
	}
	if parent == nil {
		return out, domain.QualityMissing
	}
	u, e := c.p.Users.Get(ctx, a.ID)
	if e != nil {
		return out, e
	}
	if u == nil || !u.Active || u.ID != a.ID {
		return out, domain.QualityUnauthorized
	}
	a.Superuser = a.Superuser && u.Superuser
	live := func(code string) bool { return qualityCan(a, code) && u.Allows(code) }
	if !live("projects.purchase_quality.read") {
		return out, nil
	}
	out.Scoped, e = c.scoped(ctx, a, p, "projects.purchase_quality.read", false)
	if e != nil {
		return out, e
	}
	out.Read = out.Scoped
	wa := contracts.WorkflowActor{ID: a.ID, Active: true, Superuser: a.Superuser, Permissions: a.Permissions}
	for _, n := range []int{14} {
		allowed, err := c.p.Work.CanWork(ctx, wa, p, id, n)
		if err != nil {
			var d *domain.QualityError
			err = qualityMapped(err)
			if !errors.As(err, &d) || d.Status != 422 {
				return out, err
			}
		}
		out.Work = allowed && err == nil
		taskRead, err := c.p.Work.CanReadTask(ctx, wa, p, id, n)
		if err != nil {
			return out, qualityMapped(err)
		}
		out.Read = out.Read || taskRead
	}
	out.Write = out.Read && live("projects.purchase_quality.write")
	out.Verify = out.Read && live("projects.purchase_quality.verify")

	return out, e
}

func qualityBlock(out domain.QualitySource, message string) (domain.QualitySource, error) {
	out.Available = false
	out.Blockers = append(out.Blockers, message)
	return out, nil
}
func (c *qualitySourceContext) Source(ctx context.Context, p, id string) (domain.QualitySource, error) {
	kit, current, e := c.p.Work.Inspect(ctx, p, id)
	if e != nil {
		return domain.QualitySource{}, qualityMapped(e)
	}
	out := domain.QualitySource{QualitySourceFacts: domain.QualitySourceFacts{ProjectID: p, PackageID: id}, Available: current.Available && current.Eligible, Blockers: append([]string{}, current.Blockers...)}
	for _, step := range kit.Steps {
		if step.Number == 14 && step.State == "blocked" {
			out.Blockers = append(out.Blockers, "Quality certificates have an unresolved blocker")
		}
	}
	actual, e := c.p.Reports.Issued(ctx, p, id)
	if e != nil {
		e = qualityMapped(e)
		var d *domain.QualityError
		if errors.As(e, &d) && (d.Status == 404 || d.Status == 409 || d.Status == 422) {
			return qualityBlock(out, d.Message)
		}
		return out, e
	}
	if actual == nil {
		return qualityBlock(out, "A current issued IR / IRN is required before recording quality certificates")
	}
	if actual.ProjectID != p || actual.PackageID != id {
		return domain.QualitySource{}, domain.QualityInvalid("source", "Report belongs to another project/package")
	}
	source := actual.Source
	out.QualitySourceFacts = domain.QualitySourceFacts{ProjectID: p, PackageID: id, ReportID: actual.ID, ReportVersion: actual.Version, ReportRevision: actual.Revision, IRNumber: actual.Report.IRNumber, IRNNumber: actual.Report.IRNNumber, IssueDate: actual.Report.IssueDate, Quantity: actual.Report.Quantity, UOM: actual.Report.UOM, OrderID: source.OrderID, OrderVersion: source.OrderVersion, Number: source.Number, SupplierName: source.SupplierName, Description: source.Description, InspectionID: source.InspectionID, InspectionVersion: source.InspectionVersion}
	return out, nil
}
