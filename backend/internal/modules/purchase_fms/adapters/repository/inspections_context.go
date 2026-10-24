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

type InspectionsProviders struct {
	Projects  projects.ContextReader
	Purchase  contracts.WorkspaceAccessGate
	Access    access.Gate
	Users     auth.AssignmentDirectory
	Work      contracts.WorkflowSourceReader
	Orders    contracts.OrdersReleasedReader
	Accepted  contracts.AcceptanceAcceptedReader
	Technical contracts.TechnicalClearanceReader
}
type inspectionsSourceContext struct{ p InspectionsProviders }

func NewInspectionsContext(p InspectionsProviders) *inspectionsSourceContext {
	return &inspectionsSourceContext{p}
}
func inspectionsMapped(e error) error {
	var ac *contracts.AcceptanceError
	if errors.As(e, &ac) {
		return &domain.InspectionsError{Status: ac.Status, Field: ac.Field, Message: ac.Message}
	}
	var tc *contracts.TechnicalError
	if errors.As(e, &tc) {
		return &domain.InspectionsError{Status: tc.Status, Field: tc.Field, Message: tc.Message}
	}
	var a *access.Error
	if errors.As(e, &a) {
		return &domain.InspectionsError{Status: a.Status, Field: a.Field, Message: a.Message}
	}
	var w *contracts.WorkflowError
	if errors.As(e, &w) {
		return &domain.InspectionsError{Status: w.Status, Field: w.Field, Message: w.Message}
	}
	var o *contracts.OrdersError
	if errors.As(e, &o) {
		return &domain.InspectionsError{Status: o.Status, Field: o.Field, Message: o.Message}
	}
	var p *contracts.WorkspaceInvalid
	if errors.As(e, &p) {
		return domain.InspectionsInvalid(p.Field, p.Message)
	}
	if errors.Is(e, contracts.WorkspaceErrForbidden) {
		return domain.InspectionsForbidden
	}
	if errors.Is(e, contracts.WorkspaceErrMissing) {
		return domain.InspectionsMissing
	}
	if errors.Is(e, contracts.WorkspaceErrUnauthorized) {
		return domain.InspectionsUnauthorized
	}
	return e
}
func inspectionsCan(a domain.Principal, code string) bool {
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
func (c *inspectionsSourceContext) scoped(ctx context.Context, a domain.Principal, p, code string, write bool) (bool, error) {
	v, e := c.p.Purchase.Check(ctx, contracts.WorkspacePrincipal{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	if e == nil {
		return !write || v.CanWrite, nil
	}
	e = inspectionsMapped(e)
	var d *domain.InspectionsError
	if !errors.As(e, &d) || d.Status != 403 {
		return false, e
	}
	x, e := c.p.Access.Check(ctx, access.Actor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	e = inspectionsMapped(e)
	if errors.As(e, &d) && d.Status == 403 {
		return false, nil
	}
	return e == nil && (!write || x.CanWrite), e
}
func (c *inspectionsSourceContext) Access(ctx context.Context, a domain.Principal, p, id string) (out domain.InspectionsAccess, e error) {

	parent, e := c.p.Projects.Get(ctx, p, true)
	if e != nil {
		return out, e
	}
	if parent == nil {
		return out, domain.InspectionsMissing
	}
	u, e := c.p.Users.Get(ctx, a.ID)
	if e != nil {
		return out, e
	}
	if u == nil || !u.Active || u.ID != a.ID {
		return out, domain.InspectionsUnauthorized
	}
	a.Superuser = a.Superuser && u.Superuser
	live := func(code string) bool { return inspectionsCan(a, code) && u.Allows(code) }
	if !live("projects.purchase_inspection.read") {
		return out, nil
	}
	out.Scoped, e = c.scoped(ctx, a, p, "projects.purchase_inspection.read", false)
	if e != nil {
		return out, e
	}
	out.Read = out.Scoped
	wa := contracts.WorkflowActor{ID: a.ID, Active: true, Superuser: a.Superuser, Permissions: a.Permissions}
	for _, n := range []int{12} {
		allowed, err := c.p.Work.CanWork(ctx, wa, p, id, n)
		if err != nil {
			var d *domain.InspectionsError
			err = inspectionsMapped(err)
			if !errors.As(err, &d) || d.Status != 422 {
				return out, err
			}
		}
		out.Work = allowed && err == nil
		taskRead, err := c.p.Work.CanReadTask(ctx, wa, p, id, n)
		if err != nil {
			return out, inspectionsMapped(err)
		}
		out.Read = out.Read || taskRead
	}
	out.Write = out.Read && live("projects.purchase_inspection.write")

	return out, e
}

func inspectionsBlock(out domain.InspectionsSource, message string) (domain.InspectionsSource, error) {
	out.Available = false
	out.Blockers = append(out.Blockers, message)
	return out, nil
}
func inspectionsSourceFailure(out domain.InspectionsSource, e error) (domain.InspectionsSource, error) {
	e = inspectionsMapped(e)
	var d *domain.InspectionsError
	if errors.As(e, &d) && (d.Status == 422 || d.Status == 409 || d.Status == 404) {
		return inspectionsBlock(out, d.Message)
	}
	return out, e
}
func (c *inspectionsSourceContext) Source(ctx context.Context, p, id string) (domain.InspectionsSource, error) {
	kit, current, e := c.p.Work.Inspect(ctx, p, id)
	if e != nil {
		return domain.InspectionsSource{}, inspectionsMapped(e)
	}
	out := domain.InspectionsSource{InspectionsSourceFacts: domain.InspectionsSourceFacts{ProjectID: p, PackageID: id}, Available: current.Available && current.Eligible, Blockers: append([]string{}, current.Blockers...)}
	for _, step := range kit.Steps {
		if step.Number == 12 && step.State == "blocked" {
			out.Blockers = append(out.Blockers, "Material inspection has an unresolved blocker")
		}
	}
	po, e := c.p.Orders.Released(ctx, p, id)
	if e != nil {
		return inspectionsSourceFailure(out, e)
	}
	if po == nil {
		return inspectionsBlock(out, "Release the actual PO before recording material inspection")
	}
	out.OrderID = po.ID
	out.OrderVersion = po.Version
	out.Number = po.Number
	out.Date = po.Details.Date
	out.SupplierID = po.Source.Supplier.ID
	out.SupplierName = po.Source.Supplier.Name
	out.Description = po.Source.Description
	out.Quantity = po.Source.Quantity
	out.UOM = po.Source.UOM
	acceptance, e := c.p.Accepted.Accepted(ctx, p, id)
	if e != nil {
		return inspectionsSourceFailure(out, e)
	}
	if acceptance == nil || acceptance.Order.ID != po.ID || acceptance.Order.Version != po.Version {
		return inspectionsBlock(out, "Record current unconditional vendor acceptance for this PO")
	}
	out.AcceptanceID = acceptance.ID
	out.AcceptanceVersion = acceptance.Version
	approval, e := c.p.Technical.Clearance(ctx, p, id)
	if e != nil {
		return inspectionsSourceFailure(out, e)
	}
	if approval == nil {
		return inspectionsBlock(out, "Current client-approved technical kit and QAP are required for inspection")
	}
	if approval.ProjectID != p || approval.PackageID != id {
		return domain.InspectionsSource{}, domain.InspectionsInvalid("source", "Technical evidence belongs to another inspection package")
	}
	out.TechnicalID = approval.CaseID
	out.TechnicalVersion = approval.Version
	out.TechnicalRevision = approval.Revision
	out.QAPReference = approval.Approval.Reference
	out.QAPDate = approval.Approval.Date
	out.SignedApprovalFileID = approval.Approval.FileID
	for _, doc := range approval.Documents {
		if doc.Active && doc.Kind == "qap" && doc.Status == "received" {
			out.QAPFileID = doc.FileID
			break
		}
	}
	if out.QAPFileID == "" {
		return inspectionsBlock(out, "Retain the actual approved QAP document before recording inspection")
	}
	return out, nil
}
