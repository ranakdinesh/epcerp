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

type SiteReceiptsProviders struct {
	Projects projects.ContextReader
	Purchase contracts.WorkspaceAccessGate
	Access   access.Gate
	Users    auth.AssignmentDirectory
	Work     contracts.WorkflowSourceReader
	Dispatch contracts.DispatchedReader
}
type siteReceiptsSourceContext struct{ p SiteReceiptsProviders }

func NewSiteReceiptsContext(p SiteReceiptsProviders) *siteReceiptsSourceContext {
	return &siteReceiptsSourceContext{p}
}
func siteReceiptsMapped(e error) error {
	var ic *contracts.DispatchError
	if errors.As(e, &ic) {
		return &domain.SiteReceiptsError{Status: ic.Status, Field: ic.Field, Message: ic.Message}
	}
	var a *access.Error
	if errors.As(e, &a) {
		return &domain.SiteReceiptsError{Status: a.Status, Field: a.Field, Message: a.Message}
	}
	var w *contracts.WorkflowError
	if errors.As(e, &w) {
		return &domain.SiteReceiptsError{Status: w.Status, Field: w.Field, Message: w.Message}
	}
	var p *contracts.WorkspaceInvalid
	if errors.As(e, &p) {
		return domain.SiteReceiptsInvalid(p.Field, p.Message)
	}
	if errors.Is(e, contracts.WorkspaceErrForbidden) {
		return domain.SiteReceiptsForbidden
	}
	if errors.Is(e, contracts.WorkspaceErrMissing) {
		return domain.SiteReceiptsMissing
	}
	if errors.Is(e, contracts.WorkspaceErrUnauthorized) {
		return domain.SiteReceiptsUnauthorized
	}
	return e
}
func siteReceiptsCan(a domain.Principal, code string) bool {
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
func (c *siteReceiptsSourceContext) scoped(ctx context.Context, a domain.Principal, p, code string, write bool) (bool, error) {
	v, e := c.p.Purchase.Check(ctx, contracts.WorkspacePrincipal{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	if e == nil {
		return !write || v.CanWrite, nil
	}
	e = siteReceiptsMapped(e)
	var d *domain.SiteReceiptsError
	if !errors.As(e, &d) || d.Status != 403 {
		return false, e
	}
	x, e := c.p.Access.Check(ctx, access.Actor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	e = siteReceiptsMapped(e)
	if errors.As(e, &d) && d.Status == 403 {
		return false, nil
	}
	return e == nil && (!write || x.CanWrite), e
}
func (c *siteReceiptsSourceContext) Access(ctx context.Context, a domain.Principal, p, id string) (out domain.SiteReceiptsAccess, e error) {

	parent, e := c.p.Projects.Get(ctx, p, true)
	if e != nil {
		return out, e
	}
	if parent == nil {
		return out, domain.SiteReceiptsMissing
	}
	u, e := c.p.Users.Get(ctx, a.ID)
	if e != nil {
		return out, e
	}
	if u == nil || !u.Active || u.ID != a.ID {
		return out, domain.SiteReceiptsUnauthorized
	}
	a.Superuser = a.Superuser && u.Superuser
	live := func(code string) bool { return siteReceiptsCan(a, code) && u.Allows(code) }
	if !live("projects.purchase_site_receipts.read") {
		return out, nil
	}
	out.Scoped, e = c.scoped(ctx, a, p, "projects.purchase_site_receipts.read", false)
	if e != nil {
		return out, e
	}
	out.Read = out.Scoped
	wa := contracts.WorkflowActor{ID: a.ID, Active: true, Superuser: a.Superuser, Permissions: a.Permissions}
	for _, n := range []int{17} {
		allowed, err := c.p.Work.CanWork(ctx, wa, p, id, n)
		if err != nil {
			var d *domain.SiteReceiptsError
			err = siteReceiptsMapped(err)
			if !errors.As(err, &d) || d.Status != 422 {
				return out, err
			}
		}
		out.Work = allowed && err == nil
		taskRead, err := c.p.Work.CanReadTask(ctx, wa, p, id, n)
		if err != nil {
			return out, siteReceiptsMapped(err)
		}
		out.Read = out.Read || taskRead
	}
	out.Write = out.Read && live("projects.purchase_site_receipts.write")

	return out, e
}

func siteReceiptsBlock(out domain.SiteReceiptsSource, message string) (domain.SiteReceiptsSource, error) {
	out.Available = false
	out.Blockers = append(out.Blockers, message)
	return out, nil
}
func (c *siteReceiptsSourceContext) Source(ctx context.Context, p, id string) (domain.SiteReceiptsSource, error) {
	kit, current, e := c.p.Work.Inspect(ctx, p, id)
	if e != nil {
		return domain.SiteReceiptsSource{}, siteReceiptsMapped(e)
	}
	out := domain.SiteReceiptsSource{SiteReceiptsSourceFacts: domain.SiteReceiptsSourceFacts{ProjectID: p, PackageID: id}, Available: current.Available && current.Eligible, Blockers: append([]string{}, current.Blockers...)}
	for _, step := range kit.Steps {
		if step.Number == 17 && step.State == "blocked" {
			out.Blockers = append(out.Blockers, "Site material receipt has an unresolved blocker")
		}
	}
	actual, e := c.p.Dispatch.Dispatched(ctx, p, id)
	if e != nil {
		e = siteReceiptsMapped(e)
		var d *domain.SiteReceiptsError
		if errors.As(e, &d) && (d.Status == 404 || d.Status == 409 || d.Status == 422) {
			return siteReceiptsBlock(out, d.Message)
		}
		return out, e
	}
	if actual == nil {
		return siteReceiptsBlock(out, "Current material dispatch must be recorded before site receipt")
	}
	if actual.ProjectID != p || actual.PackageID != id {
		return domain.SiteReceiptsSource{}, domain.SiteReceiptsInvalid("source", "Dispatch belongs to another project/package")
	}
	source := actual.Source
	out.SiteReceiptsSourceFacts = domain.SiteReceiptsSourceFacts{ProjectID: p, PackageID: id, DispatchID: actual.ID, DispatchVersion: actual.Version, DispatchRevision: actual.Revision, DispatchDate: actual.Dispatch.Date, LRNumber: actual.Dispatch.LRNumber, Transporter: actual.Dispatch.Transporter, ClearanceID: source.ClearanceID, ClearanceVersion: source.ClearanceVersion, ClearanceRevision: source.ClearanceRevision, Reference: source.Reference, ClearanceDate: source.ClearanceDate, QualityID: source.QualityID, QualityVersion: source.QualityVersion, QualityRevision: source.QualityRevision, ReportID: source.ReportID, ReportVersion: source.ReportVersion, ReportRevision: source.ReportRevision, IRNumber: source.IRNumber, IRNNumber: source.IRNNumber, IssueDate: source.IssueDate, OrderID: source.OrderID, OrderVersion: source.OrderVersion, Number: source.Number, SupplierName: source.SupplierName, Description: source.Description, Quantity: source.Quantity, UOM: source.UOM, InspectionID: source.InspectionID, InspectionVersion: source.InspectionVersion}
	return out, nil
}
