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

type AdvancesProviders struct {
	Projects     projects.ContextReader
	Purchase     contracts.WorkspaceAccessGate
	Access       access.Gate
	Users        auth.AssignmentDirectory
	Work         contracts.WorkflowSourceReader
	Orders       contracts.OrdersReleasedReader
	Accepted     contracts.AcceptanceAcceptedReader
	Verification masterContracts.VendorVerificationGate
	Verified     masterContracts.VendorVerificationSourceReader
}
type advancesSourceContext struct{ p AdvancesProviders }

func NewAdvancesContext(p AdvancesProviders) *advancesSourceContext { return &advancesSourceContext{p} }
func advancesMapped(e error) error {
	var a *access.Error
	if errors.As(e, &a) {
		return &domain.AdvancesError{Status: a.Status, Field: a.Field, Message: a.Message}
	}
	var w *contracts.WorkflowError
	if errors.As(e, &w) {
		return &domain.AdvancesError{Status: w.Status, Field: w.Field, Message: w.Message}
	}
	var o *contracts.OrdersError
	if errors.As(e, &o) {
		return &domain.AdvancesError{Status: o.Status, Field: o.Field, Message: o.Message}
	}
	var p *contracts.WorkspaceInvalid
	if errors.As(e, &p) {
		return domain.AdvancesInvalid(p.Field, p.Message)
	}
	if errors.Is(e, contracts.WorkspaceErrForbidden) {
		return domain.AdvancesForbidden
	}
	if errors.Is(e, contracts.WorkspaceErrMissing) {
		return domain.AdvancesMissing
	}
	if errors.Is(e, contracts.WorkspaceErrUnauthorized) {
		return domain.AdvancesUnauthorized
	}
	return e
}
func advancesCan(a domain.Principal, code string) bool {
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
func (c *advancesSourceContext) scoped(ctx context.Context, a domain.Principal, p, code string, write bool) (bool, error) {
	v, e := c.p.Purchase.Check(ctx, contracts.WorkspacePrincipal{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	if e == nil {
		return !write || v.CanWrite, nil
	}
	e = advancesMapped(e)
	var d *domain.AdvancesError
	if !errors.As(e, &d) || d.Status != 403 {
		return false, e
	}
	x, e := c.p.Access.Check(ctx, access.Actor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	e = advancesMapped(e)
	if errors.As(e, &d) && d.Status == 403 {
		return false, nil
	}
	return e == nil && (!write || x.CanWrite), e
}
func (c *advancesSourceContext) Access(ctx context.Context, a domain.Principal, p, id string) (out domain.AdvancesAccess, e error) {
	parent, e := c.p.Projects.Get(ctx, p, true)
	if e != nil {
		return out, e
	}
	if parent == nil {
		return out, domain.AdvancesMissing
	}
	user, e := c.p.Users.Get(ctx, a.ID)
	if e != nil {
		return out, e
	}
	if user == nil || !user.Active || user.ID != a.ID {
		return out, domain.AdvancesUnauthorized
	}
	a.Superuser = a.Superuser && user.Superuser
	live := func(code string) bool { return advancesCan(a, code) && user.Allows(code) }
	if live("projects.purchase_advances.read") {
		out.Scoped, e = c.scoped(ctx, a, p, "projects.purchase_advances.read", false)
		if e != nil {
			return out, e
		}
		out.Read = out.Scoped
		wa := contracts.WorkflowActor{ID: a.ID, Active: true, Superuser: a.Superuser, Permissions: a.Permissions}
		out.Work, e = c.p.Work.CanWork(ctx, wa, p, id, 11)
		if e != nil {
			var d *domain.AdvancesError
			e = advancesMapped(e)
			if !errors.As(e, &d) || d.Status != 422 {
				return out, e
			}
			out.Work = false
			e = nil
		}
		taskRead, err := c.p.Work.CanReadTask(ctx, wa, p, id, 11)
		if err != nil {
			return out, advancesMapped(err)
		}
		out.Read = out.Read || taskRead
		out.Write = out.Read && live("projects.purchase_advances.write")
	}
	if live("accounts.purchase_advances.view") && live("accounts.purchase_advances.read") {
		g, err := c.p.Access.Check(ctx, access.Actor{ID: a.ID, Active: true, Superuser: a.Superuser, Permissions: a.Permissions}, p, "accounts.purchase_advances.read", false)
		if err == nil {
			out.Read = true
			out.Scoped = true
			_ = g
			for _, code := range []string{"accounts.purchase_advances.write", "accounts.purchase_advances.disburse"} {
				if !live(code) {
					continue
				}
				grant, err := c.p.Access.Check(ctx, access.Actor{ID: a.ID, Active: true, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, true)
				if err != nil {
					var d *access.Error
					if !errors.As(err, &d) || d.Status != 403 {
						return out, advancesMapped(err)
					}
				} else if grant.CanWrite {
					if code == "accounts.purchase_advances.write" {
						out.Review = true
					} else {
						out.Disburse = true
					}
				}
			}
		} else {
			var d *access.Error
			if !errors.As(err, &d) || d.Status != 403 {
				return out, advancesMapped(err)
			}
		}
	}
	return out, nil
}
func advancesBlock(out domain.AdvancesSource, message string) (domain.AdvancesSource, error) {
	out.Available = false
	out.Blockers = append(out.Blockers, message)
	return out, nil
}
func (c *advancesSourceContext) Source(ctx context.Context, p, id string) (domain.AdvancesSource, error) {
	kit, current, e := c.p.Work.Inspect(ctx, p, id)
	if e != nil {
		return domain.AdvancesSource{}, advancesMapped(e)
	}
	out := domain.AdvancesSource{AdvancesSourceFacts: domain.AdvancesSourceFacts{ProjectID: p, PackageID: id, CalculationBasis: "po_total"}, Available: current.Available && current.Eligible, Blockers: append([]string{}, current.Blockers...)}
	for _, step := range kit.Steps {
		if step.Number == 11 && step.State == "blocked" {
			out.Blockers = append(out.Blockers, "Advance request has an unresolved blocker")
		}
	}
	po, e := c.p.Orders.Released(ctx, p, id)
	if e != nil {
		mappedErr := advancesMapped(e)
		var d *domain.AdvancesError
		if errors.As(mappedErr, &d) && (d.Status == 404 || d.Status == 422) {
			return advancesBlock(out, "Release the actual purchase order first")
		}
		return out, mappedErr
	}
	if po == nil {
		return advancesBlock(out, "Release the actual purchase order first")
	}
	out.OrderID = po.ID
	out.OrderVersion = po.Version
	out.Number = po.Number
	out.Date = po.Details.Date
	out.SupplierID = po.Source.Supplier.ID
	out.SupplierName = po.Source.Supplier.Name
	out.Currency = po.Source.Currency
	out.Total = po.Source.Total
	out.PaymentTerms = po.Source.PaymentTerms
	accepted, e := c.p.Accepted.Accepted(ctx, p, id)
	if e != nil {
		var d *contracts.AcceptanceError
		if errors.As(e, &d) && (d.Status == 404 || d.Status == 422 || d.Status == 409) {
			return advancesBlock(out, "Current unconditional vendor acceptance is required")
		}
		return out, e
	}
	if accepted == nil || accepted.Order.ID != po.ID || accepted.Order.Version != po.Version {
		return advancesBlock(out, "Current unconditional vendor acceptance is required")
	}
	out.AcceptanceID = accepted.ID
	out.AcceptanceVersion = accepted.Version
	eligible, e := c.p.Verification.Check(ctx, out.SupplierID)
	if e != nil {
		return out, e
	}
	if !eligible.CanGeneratePurchaseOrder {
		return advancesBlock(out, eligible.Message)
	}
	verified, e := c.p.Verified.Source(ctx, out.SupplierID)
	if e != nil {
		return out, e
	}
	out.SupplierVerification = verified.Fingerprint
	return out, nil
}

func (c *advancesSourceContext) FinanceRead(ctx context.Context, a domain.Principal, p string) (bool, error) {
	u, e := c.p.Users.Get(ctx, a.ID)
	if e != nil {
		return false, e
	}
	if u == nil || !u.Active || u.ID != a.ID {
		return false, domain.AdvancesUnauthorized
	}
	a.Superuser = a.Superuser && u.Superuser
	for _, code := range []string{"accounts.purchase_advances.view", "accounts.purchase_advances.read"} {
		if !a.Can(code) || !u.Allows(code) {
			return false, nil
		}
		_, e = c.p.Access.Check(ctx, access.Actor{ID: a.ID, Active: true, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, false)
		if e != nil {
			return false, advancesMapped(e)
		}
	}
	return true, nil
}
func (c *advancesSourceContext) ProjectLabel(ctx context.Context, p string) (string, string, error) {
	v, e := c.p.Projects.Get(ctx, p, false)
	if e != nil {
		return "", "", e
	}
	if v == nil {
		return "", "", domain.AdvancesMissing
	}
	return v.Name, v.Number, nil
}
