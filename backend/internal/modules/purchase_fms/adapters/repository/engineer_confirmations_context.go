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

type EngineerConfirmationsProviders struct {
	Projects projects.ContextReader
	Purchase contracts.WorkspaceAccessGate
	Access   access.Gate
	Users    auth.AssignmentDirectory
	Work     contracts.WorkflowSourceReader
	GRN      contracts.GRNsIssuedReader
}
type engineerConfirmationsSourceContext struct {
	p EngineerConfirmationsProviders
}

func NewEngineerConfirmationsContext(p EngineerConfirmationsProviders) *engineerConfirmationsSourceContext {
	return &engineerConfirmationsSourceContext{p}
}
func engineerConfirmationsMapped(e error) error {
	var ic *contracts.GRNsError
	if errors.As(e, &ic) {
		return &domain.EngineerConfirmationsError{Status: ic.Status, Field: ic.Field, Message: ic.Message}
	}
	var a *access.Error
	if errors.As(e, &a) {
		return &domain.EngineerConfirmationsError{Status: a.Status, Field: a.Field, Message: a.Message}
	}
	var w *contracts.WorkflowError
	if errors.As(e, &w) {
		return &domain.EngineerConfirmationsError{Status: w.Status, Field: w.Field, Message: w.Message}
	}
	var p *contracts.WorkspaceInvalid
	if errors.As(e, &p) {
		return domain.EngineerConfirmationsInvalid(p.Field, p.Message)
	}
	if errors.Is(e, contracts.WorkspaceErrForbidden) {
		return domain.EngineerConfirmationsForbidden
	}
	if errors.Is(e, contracts.WorkspaceErrMissing) {
		return domain.EngineerConfirmationsMissing
	}
	if errors.Is(e, contracts.WorkspaceErrUnauthorized) {
		return domain.EngineerConfirmationsUnauthorized
	}
	return e
}
func engineerConfirmationsCan(a domain.Principal, code string) bool {
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
func (c *engineerConfirmationsSourceContext) scoped(ctx context.Context, a domain.Principal, p, code string, write bool) (bool, error) {
	v, e := c.p.Purchase.Check(ctx, contracts.WorkspacePrincipal{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	if e == nil {
		return !write || v.CanWrite, nil
	}
	e = engineerConfirmationsMapped(e)
	var d *domain.EngineerConfirmationsError
	if !errors.As(e, &d) || d.Status != 403 {
		return false, e
	}
	x, e := c.p.Access.Check(ctx, access.Actor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	e = engineerConfirmationsMapped(e)
	if errors.As(e, &d) && d.Status == 403 {
		return false, nil
	}
	return e == nil && (!write || x.CanWrite), e
}
func (c *engineerConfirmationsSourceContext) Access(ctx context.Context, a domain.Principal, p, id string) (out domain.EngineerConfirmationsAccess, e error) {

	parent, e := c.p.Projects.Get(ctx, p, true)
	if e != nil {
		return out, e
	}
	if parent == nil {
		return out, domain.EngineerConfirmationsMissing
	}
	u, e := c.p.Users.Get(ctx, a.ID)
	if e != nil {
		return out, e
	}
	if u == nil || !u.Active || u.ID != a.ID {
		return out, domain.EngineerConfirmationsUnauthorized
	}
	a.Superuser = a.Superuser && u.Superuser
	live := func(code string) bool { return engineerConfirmationsCan(a, code) && u.Allows(code) }
	if !live("projects.purchase_engineer_confirmations.read") {
		return out, nil
	}
	out.Scoped, e = c.scoped(ctx, a, p, "projects.purchase_engineer_confirmations.read", false)
	if e != nil {
		return out, e
	}
	out.Read = out.Scoped
	wa := contracts.WorkflowActor{ID: a.ID, Active: true, Superuser: a.Superuser, Permissions: a.Permissions}
	for _, n := range []int{19} {
		allowed, err := c.p.Work.CanWork(ctx, wa, p, id, n)
		if err != nil {
			var d *domain.EngineerConfirmationsError
			err = engineerConfirmationsMapped(err)
			if !errors.As(err, &d) || d.Status != 422 {
				return out, err
			}
		}
		out.Work = allowed && err == nil
		taskRead, err := c.p.Work.CanReadTask(ctx, wa, p, id, n)
		if err != nil {
			return out, engineerConfirmationsMapped(err)
		}
		out.Read = out.Read || taskRead
	}
	out.Write = out.Read && live("projects.purchase_engineer_confirmations.write")

	return out, e
}

func engineerConfirmationsBlock(out domain.EngineerConfirmationsSource, message string) (domain.EngineerConfirmationsSource, error) {
	out.Available = false
	out.Blockers = append(out.Blockers, message)
	return out, nil
}
func (c *engineerConfirmationsSourceContext) Source(ctx context.Context, p, id string) (domain.EngineerConfirmationsSource, error) {
	kit, current, e := c.p.Work.Inspect(ctx, p, id)
	if e != nil {
		return domain.EngineerConfirmationsSource{}, engineerConfirmationsMapped(e)
	}
	out := domain.EngineerConfirmationsSource{EngineerConfirmationsSourceFacts: domain.EngineerConfirmationsSourceFacts{ProjectID: p, PackageID: id}, Available: current.Available && current.Eligible, Blockers: append([]string{}, current.Blockers...)}
	for _, step := range kit.Steps {
		if step.Number == 19 && step.State == "blocked" {
			out.Blockers = append(out.Blockers, "Engineer confirmation has an unresolved blocker")
		}
	}
	actual, e := c.p.GRN.Issued(ctx, p, id)
	if e != nil {
		e = engineerConfirmationsMapped(e)
		var d *domain.EngineerConfirmationsError
		if errors.As(e, &d) && (d.Status == 404 || d.Status == 409 || d.Status == 422) {
			return engineerConfirmationsBlock(out, d.Message)
		}
		return out, e
	}
	if actual == nil {
		return engineerConfirmationsBlock(out, "Current issued GRN is required before engineer confirmation")
	}
	if actual.ProjectID != p || actual.PackageID != id {
		return domain.EngineerConfirmationsSource{}, domain.EngineerConfirmationsInvalid("source", "GRN belongs to another project/package")
	}
	source := actual.Source
	out.EngineerConfirmationsSourceFacts = domain.EngineerConfirmationsSourceFacts{ProjectID: p, PackageID: id, GRNID: actual.ID, GRNVersion: actual.Version, GRNRevision: actual.Revision, GRNNumber: actual.GRN.Number, GRNDate: actual.GRN.Date, Received: actual.GRN.Received, Accepted: actual.GRN.Accepted, Rejected: actual.GRN.Rejected, WarrantyReference: actual.GRN.WarrantyReference, WarrantyValidUntil: actual.GRN.WarrantyValidUntil, ReceiptID: source.ReceiptID, ReceiptVersion: source.ReceiptVersion, ReceiptRevision: source.ReceiptRevision, ArrivalDate: source.ArrivalDate, ReceivingOfficer: source.ReceivingOfficer, PackagesReceived: source.PackagesReceived, PackagesCondition: source.PackagesCondition, TransitDamageNotes: source.TransitDamageNotes, GateEntryReference: source.GateEntryReference, DispatchID: source.DispatchID, DispatchVersion: source.DispatchVersion, DispatchRevision: source.DispatchRevision, DispatchDate: source.DispatchDate, LRNumber: source.LRNumber, Transporter: source.Transporter, ClearanceID: source.ClearanceID, ClearanceVersion: source.ClearanceVersion, ClearanceRevision: source.ClearanceRevision, Reference: source.Reference, ClearanceDate: source.ClearanceDate, QualityID: source.QualityID, QualityVersion: source.QualityVersion, QualityRevision: source.QualityRevision, ReportID: source.ReportID, ReportVersion: source.ReportVersion, ReportRevision: source.ReportRevision, IRNumber: source.IRNumber, IRNNumber: source.IRNNumber, IssueDate: source.IssueDate, OrderID: source.OrderID, OrderVersion: source.OrderVersion, Number: source.Number, SupplierName: source.SupplierName, Description: source.Description, Quantity: source.Quantity, UOM: source.UOM, InspectionID: source.InspectionID, InspectionVersion: source.InspectionVersion}
	return out, nil
}
