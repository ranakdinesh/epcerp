package repository

import (
	"context"
	"errors"

	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	access "github.com/ranakdinesh/epcERP/internal/modules/project_access/contracts"
	projects "github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type OrdersProviders struct {
	Projects     projects.ContextReader
	Purchase     contracts.WorkspaceAccessGate
	Access       access.Gate
	Users        auth.AssignmentDirectory
	Work         contracts.WorkflowSourceReader
	Authority    contracts.SourcingAuthorityReader
	Technical    contracts.TechnicalClearanceReader
	Verification masterContracts.VendorVerificationGate
	Verified     masterContracts.VendorVerificationSourceReader
	Suppliers    masterContracts.VendorPurchaseDirectory
	Indents      contracts.IndentsReader
}
type ordersSourceContext struct{ p OrdersProviders }

func NewOrdersContext(p OrdersProviders) *ordersSourceContext { return &ordersSourceContext{p} }
func ordersMapped(e error) error {
	var a *access.Error
	if errors.As(e, &a) {
		return &domain.OrdersError{Status: a.Status, Field: a.Field, Message: a.Message}
	}
	var w *contracts.WorkflowError
	if errors.As(e, &w) {
		return &domain.OrdersError{Status: w.Status, Field: w.Field, Message: w.Message}
	}
	var t *contracts.TechnicalError
	if errors.As(e, &t) {
		return &domain.OrdersError{Status: t.Status, Field: t.Field, Message: t.Message}
	}
	var s *contracts.SourcingError
	if errors.As(e, &s) {
		return &domain.OrdersError{Status: s.Status, Field: s.Field, Message: s.Message}
	}
	var p *contracts.WorkspaceInvalid
	if errors.As(e, &p) {
		return domain.OrdersInvalid(p.Field, p.Message)
	}
	if errors.Is(e, contracts.WorkspaceErrForbidden) {
		return domain.OrdersForbidden
	}
	if errors.Is(e, contracts.WorkspaceErrMissing) {
		return domain.OrdersMissing
	}
	if errors.Is(e, contracts.WorkspaceErrUnauthorized) {
		return domain.OrdersUnauthorized
	}
	return e
}
func ordersCan(a domain.Principal, code string) bool {
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
func (c *ordersSourceContext) scoped(ctx context.Context, a domain.Principal, p, code string, write bool) (bool, error) {
	v, e := c.p.Purchase.Check(ctx, contracts.WorkspacePrincipal{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	if e == nil {
		return !write || v.CanWrite, nil
	}
	e = ordersMapped(e)
	var d *domain.OrdersError
	if !errors.As(e, &d) || d.Status != 403 {
		return false, e
	}
	x, e := c.p.Access.Check(ctx, access.Actor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	e = ordersMapped(e)
	if errors.As(e, &d) && d.Status == 403 {
		return false, nil
	}
	return e == nil && (!write || x.CanWrite), e
}
func (c *ordersSourceContext) Access(ctx context.Context, a domain.Principal, p, id string) (out domain.OrdersAccess, e error) {

	parent, e := c.p.Projects.Get(ctx, p, true)
	if e != nil {
		return out, e
	}
	if parent == nil {
		return out, domain.OrdersMissing
	}
	u, e := c.p.Users.Get(ctx, a.ID)
	if e != nil {
		return out, e
	}
	if u == nil || !u.Active {
		return out, domain.OrdersUnauthorized
	}
	a.Superuser = a.Superuser && u.Superuser
	live := func(code string) bool { return ordersCan(a, code) && u.Allows(code) }
	if !live("projects.purchase_orders.read") {
		return out, nil
	}
	out.Scoped, e = c.scoped(ctx, a, p, "projects.purchase_orders.read", false)
	if e != nil {
		return out, e
	}
	out.Read = out.Scoped
	wa := contracts.WorkflowActor{ID: a.ID, Active: true, Superuser: a.Superuser, Permissions: a.Permissions}
	for _, n := range []int{9} {
		allowed, err := c.p.Work.CanWork(ctx, wa, p, id, n)
		if err != nil {
			var d *domain.OrdersError
			err = ordersMapped(err)
			if !errors.As(err, &d) || d.Status != 422 {
				return out, err
			}
		}
		out.Work = allowed
		taskRead, err := c.p.Work.CanReadTask(ctx, wa, p, id, n)
		if err != nil {
			return out, ordersMapped(err)
		}
		out.Read = out.Read || taskRead
	}
	out.Write = out.Read && live("projects.purchase_orders.write")
	out.Release = out.Write && out.Work && live("projects.purchase_orders.release")
	out.RecordClient = out.Write && out.Work && live("projects.purchase_orders.client_approval")

	return out, e
}

func ordersBlock(out domain.OrdersSource, message string) (domain.OrdersSource, error) {
	out.Available = false
	out.Blockers = append(out.Blockers, message)
	return out, nil
}
func ordersSourceFailure(out domain.OrdersSource, e error) (domain.OrdersSource, error) {
	e = ordersMapped(e)
	var d *domain.OrdersError
	if errors.As(e, &d) && d.Status == 422 {
		return ordersBlock(out, d.Message)
	}
	return out, e
}
func ordersField(v any) string { s, _ := v.(string); return s }
func (c *ordersSourceContext) Source(ctx context.Context, p, id string) (domain.OrdersSource, error) {
	kit, current, e := c.p.Work.Inspect(ctx, p, id)
	if e != nil {
		return domain.OrdersSource{}, ordersMapped(e)
	}
	out := domain.OrdersSource{OrdersSourceFacts: domain.OrdersSourceFacts{ProjectID: p, PackageID: id, ProjectName: current.ProjectName, Site: current.Site, Description: current.Description, Quantity: current.Quantity, UOM: current.UOM}, Available: current.Available && current.Eligible, Blockers: append([]string{}, current.Blockers...)}
	for _, step := range kit.Steps {
		if step.Number == 9 && step.State == "blocked" {
			out.Blockers = append(out.Blockers, "Purchase-order release has an unresolved blocker")
		}
	}
	loi, e := c.p.Authority.CurrentReleased(ctx, p, id)
	if e != nil {
		return ordersSourceFailure(out, e)
	}
	if loi == nil {
		return ordersBlock(out, "Release a current independently approved LOI before preparing the order")
	}
	out.SourcingID = loi.CaseID
	out.SourcingVersion = loi.Version
	out.LOIReference = loi.LOIReference
	out.LOIDate = domain.OrdersToday(loi.ReleasedAt)
	out.FinancialReference = loi.FinancialReference
	out.Currency = loi.Source.Currency
	out.Quantity = loi.Source.Quantity
	out.Description = loi.Source.Description
	out.Total = loi.Total
	offer := loi.Offer
	out.PricingMode = offer.PricingMode
	if out.PricingMode == "" {
		out.PricingMode = "unit_price"
	}
	out.UnitPrice = offer.UnitPrice
	out.Discount = offer.Discount
	out.Tax = offer.Tax
	out.Freight = offer.Freight
	out.Other = offer.Other
	out.DeliveryDays = offer.DeliveryDays
	out.PaymentTerms = offer.PaymentTerms
	out.Warranty = offer.Warranty
	out.LOITerms = loi.Terms
	out.Base, e = domain.OrdersBasicAmount(out.PricingMode, offer.UnitPrice, offer.LineTotal, out.Quantity)
	if e != nil {
		return out, e
	}
	rows, e := c.p.Suppliers.Candidates(ctx)
	if e != nil {
		return out, e
	}
	for _, v := range rows {
		if v.ID == offer.SupplierID && v.Active {
			out.Supplier = domain.OrdersSupplier{ID: v.ID, Name: v.Name, Code: v.Code, Address: ordersField(v.Data["location"]), Contact: ordersField(v.Data["contact"]), Email: ordersField(v.Data["email"]), Phone: ordersField(v.Data["phone"]), TaxReference: ordersField(v.Data["gst_number"]), Registration: ordersField(v.Data["registration_reference"])}
		}
	}
	if out.Supplier.ID == "" {
		return ordersBlock(out, "The selected supplier is no longer active")
	}
	eligibility, e := c.p.Verification.Check(ctx, out.Supplier.ID)
	if e != nil {
		return out, e
	}
	if !eligibility.CanGeneratePurchaseOrder {
		return ordersBlock(out, eligibility.Message)
	}
	verified, e := c.p.Verified.Source(ctx, out.Supplier.ID)
	if e != nil {
		return out, e
	}
	out.SupplierVerification = verified.Fingerprint
	approved, e := c.p.Technical.Clearance(ctx, p, id)
	if e != nil {
		return ordersSourceFailure(out, e)
	}
	if approved == nil {
		return ordersBlock(out, "Complete the client-approved technical kit and signed QAP before preparing the PO")
	}
	if approved.ProjectID != p || approved.PackageID != id || approved.Source.CaseID != loi.CaseID || approved.Source.Version != loi.Version || approved.Source.SupplierID != offer.SupplierID || approved.Source.FinancialReference != loi.FinancialReference || approved.Source.Quantity != loi.Source.Quantity || approved.Source.Total != loi.Total {
		return ordersBlock(out, "Technical clearance does not match the exact current commercial LOI")
	}
	out.Technical = domain.OrdersTechnical{TransmittalReference: approved.TransmittalReference, CaseID: approved.CaseID, Version: approved.Version, Revision: approved.Revision, Reference: approved.Approval.Reference, ApprovalDate: approved.Approval.Date, ApprovalFileID: approved.Approval.FileID, QAPApproved: approved.Approval.QAPApproved, ConditionsClosed: approved.Approval.Status == "approved" || approved.Approval.ConditionsClosed}
	// The transmittal is retained by the technical owner; approval identity and file
	// bind this order to the actual approved revision rather than a filename.
	indent, e := c.p.Indents.Get(ctx, p, kit.IndentID)
	if e != nil {
		return ordersSourceFailure(out, e)
	}
	if indent == nil {
		return ordersBlock(out, "Released item evidence is unavailable")
	}
	for _, line := range indent.Data.Released {
		if line.DemandID == kit.DemandID {
			out.ItemID = line.Source.Item.ID
			out.SKU = line.Source.Item.SKU
			out.UOM = line.Source.Item.UOM
		}
	}
	return out, nil
}
