package outbound

import (
	"context"
	"errors"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/domain"
	access "github.com/ranakdinesh/epcERP/internal/modules/project_access/contracts"
	projects "github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"
	purchaseContracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
)

type AuthBridge struct{ r auth.Resolver }

func NewAuthBridge(r auth.Resolver) *AuthBridge { return &AuthBridge{r} }
func (b *AuthBridge) Resolve(ctx context.Context, token string) (domain.Actor, error) {
	a, e := b.r.Resolve(ctx, token)
	if e != nil {
		return domain.Actor{}, domain.Unauthorized
	}
	return domain.Actor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: append([]string{}, a.Permissions...)}, nil
}

type Providers struct {
	Projects projects.ContextReader
	Purchase purchaseContracts.WorkspaceAccessGate
	Access   access.Gate
	Users    auth.AssignmentDirectory
	Work     purchaseContracts.WorkflowSourceReader
	Engineer purchaseContracts.EngineerConfirmationsConfirmedReader
	Orders   purchaseContracts.OrdersReleasedReader
}
type sourceContext struct{ p Providers }

func NewContext(p Providers) *sourceContext { return &sourceContext{p} }
func mapped(e error) error {
	var ic *purchaseContracts.EngineerConfirmationsError
	if errors.As(e, &ic) {
		return &domain.Error{Status: ic.Status, Field: ic.Field, Message: ic.Message}
	}
	var a *access.Error
	if errors.As(e, &a) {
		return &domain.Error{Status: a.Status, Field: a.Field, Message: a.Message}
	}
	var w *purchaseContracts.WorkflowError
	if errors.As(e, &w) {
		return &domain.Error{Status: w.Status, Field: w.Field, Message: w.Message}
	}
	var p *purchaseContracts.WorkspaceInvalid
	if errors.As(e, &p) {
		return domain.Invalid(p.Field, p.Message)
	}
	if errors.Is(e, purchaseContracts.WorkspaceErrForbidden) {
		return domain.Forbidden
	}
	if errors.Is(e, purchaseContracts.WorkspaceErrMissing) {
		return domain.Missing
	}
	if errors.Is(e, purchaseContracts.WorkspaceErrUnauthorized) {
		return domain.Unauthorized
	}
	return e
}
func can(a domain.Actor, code string) bool {
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
func (c *sourceContext) financeScope(ctx context.Context, a domain.Actor, p, code string, write bool) (bool, error) {
	x, e := c.p.Access.Check(ctx, access.Actor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	e = mapped(e)
	var d *domain.Error
	if errors.As(e, &d) && d.Status == 403 {
		return false, nil
	}
	return e == nil && (!write || x.CanWrite), e
}
func (c *sourceContext) Access(ctx context.Context, a domain.Actor, p, id string) (out domain.Access, e error) {

	parent, e := c.p.Projects.Get(ctx, p, true)
	if e != nil {
		return out, e
	}
	if parent == nil {
		return out, domain.Missing
	}
	u, e := c.p.Users.Get(ctx, a.ID)
	if e != nil {
		return out, e
	}
	if u == nil || !u.Active || u.ID != a.ID {
		return out, domain.Unauthorized
	}
	a.Superuser = a.Superuser && u.Superuser
	live := func(code string) bool { return can(a, code) && u.Allows(code) }
	if !live("accounts.purchase_acceptance.read") {
		return out, nil
	}
	if !live("accounts.purchase_acceptance.view") {
		return out, nil
	}
	out.Scoped, e = c.financeScope(ctx, a, p, "accounts.purchase_acceptance.read", false)
	if e != nil {
		return out, e
	}
	out.Read = out.Scoped
	if !out.Read {
		task, e := c.p.Work.CanReadTask(ctx, purchaseContracts.WorkflowActor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, id, 20)
		if e != nil {
			return out, mapped(e)
		}
		out.Read = task
	}
	if live("accounts.purchase_acceptance.write") {
		out.Work, e = c.financeScope(ctx, a, p, "accounts.purchase_acceptance.write", true)
		if e != nil {
			return out, e
		}
		out.Write = out.Read && out.Work
	}

	return out, e
}

func block(out domain.Source, message string) (domain.Source, error) {
	out.Available = false
	out.Blockers = append(out.Blockers, message)
	return out, nil
}
func (c *sourceContext) Source(ctx context.Context, p, id string) (domain.Source, error) {
	kit, current, e := c.p.Work.Inspect(ctx, p, id)
	if e != nil {
		return domain.Source{}, mapped(e)
	}
	out := domain.Source{SourceFacts: domain.SourceFacts{ProjectID: p, PackageID: id}, Available: current.Available && current.Eligible, Blockers: append([]string{}, current.Blockers...)}
	for _, step := range kit.Steps {
		if step.Number == 20 && step.State == "blocked" {
			out.Blockers = append(out.Blockers, "Accounts acceptance has an unresolved blocker")
		}
	}
	actual, e := c.p.Engineer.Confirmed(ctx, p, id)
	if e != nil {
		e = mapped(e)
		var d *domain.Error
		if errors.As(e, &d) && (d.Status == 404 || d.Status == 409 || d.Status == 422) {
			return block(out, d.Message)
		}
		return out, e
	}
	if actual == nil {
		return block(out, "Current engineer confirmation is required before Accounts acceptance")
	}
	if actual.ProjectID != p || actual.PackageID != id {
		return domain.Source{}, domain.Invalid("source", "GRN belongs to another project/package")
	}
	source := actual.Source
	released, e := c.p.Orders.Released(ctx, p, id)
	if e != nil {
		return out, e
	}
	if released == nil || released.ID != source.OrderID || released.Version != source.OrderVersion {
		return block(out, "Current released PO is required")
	}

	out.SourceFacts = domain.SourceFacts{ProjectID: p, PackageID: id, EngineerID: actual.ID, EngineerVersion: actual.Version, EngineerRevision: actual.Revision, ConfirmationDate: actual.Confirmation.Date, EngineerName: actual.Confirmation.EngineerName, Designation: actual.Confirmation.Designation, MBookReference: actual.Confirmation.MBookReference, WorkPackageReference: actual.Confirmation.WorkPackageReference, EngineerRemarks: actual.Confirmation.Remarks, Currency: released.Source.Currency, GRNID: source.GRNID, GRNVersion: source.GRNVersion, GRNRevision: source.GRNRevision, GRNNumber: source.GRNNumber, GRNDate: source.GRNDate, Received: source.Received, Accepted: source.Accepted, Rejected: source.Rejected, WarrantyReference: source.WarrantyReference, WarrantyValidUntil: source.WarrantyValidUntil, ReceiptID: source.ReceiptID, ReceiptVersion: source.ReceiptVersion, ReceiptRevision: source.ReceiptRevision, ArrivalDate: source.ArrivalDate, ReceivingOfficer: source.ReceivingOfficer, PackagesReceived: source.PackagesReceived, PackagesCondition: source.PackagesCondition, TransitDamageNotes: source.TransitDamageNotes, GateEntryReference: source.GateEntryReference, DispatchID: source.DispatchID, DispatchVersion: source.DispatchVersion, DispatchRevision: source.DispatchRevision, DispatchDate: source.DispatchDate, LRNumber: source.LRNumber, Transporter: source.Transporter, ClearanceID: source.ClearanceID, ClearanceVersion: source.ClearanceVersion, ClearanceRevision: source.ClearanceRevision, Reference: source.Reference, ClearanceDate: source.ClearanceDate, QualityID: source.QualityID, QualityVersion: source.QualityVersion, QualityRevision: source.QualityRevision, ReportID: source.ReportID, ReportVersion: source.ReportVersion, ReportRevision: source.ReportRevision, IRNumber: source.IRNumber, IRNNumber: source.IRNNumber, IssueDate: source.IssueDate, OrderID: source.OrderID, OrderVersion: source.OrderVersion, Number: source.Number, SupplierName: source.SupplierName, Description: source.Description, Quantity: source.Quantity, UOM: source.UOM, InspectionID: source.InspectionID, InspectionVersion: source.InspectionVersion}
	return out, nil
}
