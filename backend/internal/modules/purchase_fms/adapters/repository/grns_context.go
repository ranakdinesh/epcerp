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

type GRNsProviders struct {
	Projects projects.ContextReader
	Purchase contracts.WorkspaceAccessGate
	Access   access.Gate
	Users    auth.AssignmentDirectory
	Work     contracts.WorkflowSourceReader
	Receipt  contracts.SiteReceiptsDeliveredReader
}
type gRNsSourceContext struct{ p GRNsProviders }

func NewGRNsContext(p GRNsProviders) *gRNsSourceContext { return &gRNsSourceContext{p} }
func gRNsMapped(e error) error {
	var ic *contracts.SiteReceiptsError
	if errors.As(e, &ic) {
		return &domain.GRNsError{Status: ic.Status, Field: ic.Field, Message: ic.Message}
	}
	var a *access.Error
	if errors.As(e, &a) {
		return &domain.GRNsError{Status: a.Status, Field: a.Field, Message: a.Message}
	}
	var w *contracts.WorkflowError
	if errors.As(e, &w) {
		return &domain.GRNsError{Status: w.Status, Field: w.Field, Message: w.Message}
	}
	var p *contracts.WorkspaceInvalid
	if errors.As(e, &p) {
		return domain.GRNsInvalid(p.Field, p.Message)
	}
	if errors.Is(e, contracts.WorkspaceErrForbidden) {
		return domain.GRNsForbidden
	}
	if errors.Is(e, contracts.WorkspaceErrMissing) {
		return domain.GRNsMissing
	}
	if errors.Is(e, contracts.WorkspaceErrUnauthorized) {
		return domain.GRNsUnauthorized
	}
	return e
}
func gRNsCan(a domain.Principal, code string) bool {
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
func (c *gRNsSourceContext) scoped(ctx context.Context, a domain.Principal, p, code string, write bool) (bool, error) {
	v, e := c.p.Purchase.Check(ctx, contracts.WorkspacePrincipal{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	if e == nil {
		return !write || v.CanWrite, nil
	}
	e = gRNsMapped(e)
	var d *domain.GRNsError
	if !errors.As(e, &d) || d.Status != 403 {
		return false, e
	}
	x, e := c.p.Access.Check(ctx, access.Actor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	e = gRNsMapped(e)
	if errors.As(e, &d) && d.Status == 403 {
		return false, nil
	}
	return e == nil && (!write || x.CanWrite), e
}
func (c *gRNsSourceContext) Access(ctx context.Context, a domain.Principal, p, id string) (out domain.GRNsAccess, e error) {

	parent, e := c.p.Projects.Get(ctx, p, true)
	if e != nil {
		return out, e
	}
	if parent == nil {
		return out, domain.GRNsMissing
	}
	u, e := c.p.Users.Get(ctx, a.ID)
	if e != nil {
		return out, e
	}
	if u == nil || !u.Active || u.ID != a.ID {
		return out, domain.GRNsUnauthorized
	}
	a.Superuser = a.Superuser && u.Superuser
	live := func(code string) bool { return gRNsCan(a, code) && u.Allows(code) }
	if !live("projects.purchase_grns.read") {
		return out, nil
	}
	out.Scoped, e = c.scoped(ctx, a, p, "projects.purchase_grns.read", false)
	if e != nil {
		return out, e
	}
	out.Read = out.Scoped
	wa := contracts.WorkflowActor{ID: a.ID, Active: true, Superuser: a.Superuser, Permissions: a.Permissions}
	for _, n := range []int{18} {
		allowed, err := c.p.Work.CanWork(ctx, wa, p, id, n)
		if err != nil {
			var d *domain.GRNsError
			err = gRNsMapped(err)
			if !errors.As(err, &d) || d.Status != 422 {
				return out, err
			}
		}
		out.Work = allowed && err == nil
		taskRead, err := c.p.Work.CanReadTask(ctx, wa, p, id, n)
		if err != nil {
			return out, gRNsMapped(err)
		}
		out.Read = out.Read || taskRead
	}
	out.Write = out.Read && live("projects.purchase_grns.write")

	return out, e
}

func gRNsBlock(out domain.GRNsSource, message string) (domain.GRNsSource, error) {
	out.Available = false
	out.Blockers = append(out.Blockers, message)
	return out, nil
}
func (c *gRNsSourceContext) Source(ctx context.Context, p, id string) (domain.GRNsSource, error) {
	kit, current, e := c.p.Work.Inspect(ctx, p, id)
	if e != nil {
		return domain.GRNsSource{}, gRNsMapped(e)
	}
	out := domain.GRNsSource{GRNsSourceFacts: domain.GRNsSourceFacts{ProjectID: p, PackageID: id}, Available: current.Available && current.Eligible, Blockers: append([]string{}, current.Blockers...)}
	for _, step := range kit.Steps {
		if step.Number == 18 && step.State == "blocked" {
			out.Blockers = append(out.Blockers, "GRN inwarding has an unresolved blocker")
		}
	}
	actual, e := c.p.Receipt.Delivered(ctx, p, id)
	if e != nil {
		e = gRNsMapped(e)
		var d *domain.GRNsError
		if errors.As(e, &d) && (d.Status == 404 || d.Status == 409 || d.Status == 422) {
			return gRNsBlock(out, d.Message)
		}
		return out, e
	}
	if actual == nil {
		return gRNsBlock(out, "Current physical site receipt must be delivered before GRN inwarding")
	}
	if actual.ProjectID != p || actual.PackageID != id {
		return domain.GRNsSource{}, domain.GRNsInvalid("source", "Receipt belongs to another project/package")
	}
	source := actual.Source
	out.GRNsSourceFacts = domain.GRNsSourceFacts{ProjectID: p, PackageID: id, ReceiptID: actual.ID, ReceiptVersion: actual.Version, ReceiptRevision: actual.Revision, ArrivalDate: actual.Receipt.ArrivalDate, ReceivingOfficer: actual.Receipt.ReceivingOfficer, PackagesReceived: actual.Receipt.PackagesReceived, PackagesCondition: actual.Receipt.PackagesCondition, TransitDamageNotes: actual.Receipt.TransitDamageNotes, GateEntryReference: actual.Receipt.GateEntryReference, DispatchID: source.DispatchID, DispatchVersion: source.DispatchVersion, DispatchRevision: source.DispatchRevision, DispatchDate: source.DispatchDate, LRNumber: source.LRNumber, Transporter: source.Transporter, ClearanceID: source.ClearanceID, ClearanceVersion: source.ClearanceVersion, ClearanceRevision: source.ClearanceRevision, Reference: source.Reference, ClearanceDate: source.ClearanceDate, QualityID: source.QualityID, QualityVersion: source.QualityVersion, QualityRevision: source.QualityRevision, ReportID: source.ReportID, ReportVersion: source.ReportVersion, ReportRevision: source.ReportRevision, IRNumber: source.IRNumber, IRNNumber: source.IRNNumber, IssueDate: source.IssueDate, OrderID: source.OrderID, OrderVersion: source.OrderVersion, Number: source.Number, SupplierName: source.SupplierName, Description: source.Description, Quantity: source.Quantity, UOM: source.UOM, InspectionID: source.InspectionID, InspectionVersion: source.InspectionVersion}
	return out, nil
}
