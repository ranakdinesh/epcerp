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

type AcceptanceProviders struct {
	Projects projects.ContextReader
	Purchase contracts.WorkspaceAccessGate
	Access   access.Gate
	Users    auth.AssignmentDirectory
	Work     contracts.WorkflowSourceReader
	Orders   contracts.OrdersReleasedReader
}
type acceptanceSourceContext struct{ p AcceptanceProviders }

func NewAcceptanceContext(p AcceptanceProviders) *acceptanceSourceContext {
	return &acceptanceSourceContext{p}
}
func acceptanceMapped(e error) error {
	var a *access.Error
	if errors.As(e, &a) {
		return &domain.AcceptanceError{Status: a.Status, Field: a.Field, Message: a.Message}
	}
	var w *contracts.WorkflowError
	if errors.As(e, &w) {
		return &domain.AcceptanceError{Status: w.Status, Field: w.Field, Message: w.Message}
	}
	var o *contracts.OrdersError
	if errors.As(e, &o) {
		return &domain.AcceptanceError{Status: o.Status, Field: o.Field, Message: o.Message}
	}
	var p *contracts.WorkspaceInvalid
	if errors.As(e, &p) {
		return domain.AcceptanceInvalid(p.Field, p.Message)
	}
	if errors.Is(e, contracts.WorkspaceErrForbidden) {
		return domain.AcceptanceForbidden
	}
	if errors.Is(e, contracts.WorkspaceErrMissing) {
		return domain.AcceptanceMissing
	}
	if errors.Is(e, contracts.WorkspaceErrUnauthorized) {
		return domain.AcceptanceUnauthorized
	}
	return e
}
func acceptanceCan(a domain.Principal, code string) bool {
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
func (c *acceptanceSourceContext) scoped(ctx context.Context, a domain.Principal, p, code string, write bool) (bool, error) {
	v, e := c.p.Purchase.Check(ctx, contracts.WorkspacePrincipal{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	if e == nil {
		return !write || v.CanWrite, nil
	}
	e = acceptanceMapped(e)
	var d *domain.AcceptanceError
	if !errors.As(e, &d) || d.Status != 403 {
		return false, e
	}
	x, e := c.p.Access.Check(ctx, access.Actor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	e = acceptanceMapped(e)
	if errors.As(e, &d) && d.Status == 403 {
		return false, nil
	}
	return e == nil && (!write || x.CanWrite), e
}
func (c *acceptanceSourceContext) Access(ctx context.Context, a domain.Principal, p, id string) (out domain.AcceptanceAccess, e error) {

	parent, e := c.p.Projects.Get(ctx, p, true)
	if e != nil {
		return out, e
	}
	if parent == nil {
		return out, domain.AcceptanceMissing
	}
	u, e := c.p.Users.Get(ctx, a.ID)
	if e != nil {
		return out, e
	}
	if u == nil || !u.Active {
		return out, domain.AcceptanceUnauthorized
	}
	a.Superuser = a.Superuser && u.Superuser
	live := func(code string) bool { return acceptanceCan(a, code) && u.Allows(code) }
	if !live("projects.purchase_acceptance.read") {
		return out, nil
	}
	out.Scoped, e = c.scoped(ctx, a, p, "projects.purchase_acceptance.read", false)
	if e != nil {
		return out, e
	}
	out.Read = out.Scoped
	wa := contracts.WorkflowActor{ID: a.ID, Active: true, Superuser: a.Superuser, Permissions: a.Permissions}
	for _, n := range []int{10} {
		allowed, err := c.p.Work.CanWork(ctx, wa, p, id, n)
		if err != nil {
			var d *domain.AcceptanceError
			err = acceptanceMapped(err)
			if !errors.As(err, &d) || d.Status != 422 {
				return out, err
			}
		}
		out.Work = allowed
		taskRead, err := c.p.Work.CanReadTask(ctx, wa, p, id, n)
		if err != nil {
			return out, acceptanceMapped(err)
		}
		out.Read = out.Read || taskRead
	}
	out.Write = out.Read && live("projects.purchase_acceptance.write")

	return out, e
}

func acceptanceBlock(out domain.AcceptanceSource, message string) (domain.AcceptanceSource, error) {
	out.Available = false
	out.Blockers = append(out.Blockers, message)
	return out, nil
}
func acceptanceSourceFailure(out domain.AcceptanceSource, e error) (domain.AcceptanceSource, error) {
	e = acceptanceMapped(e)
	var d *domain.AcceptanceError
	if errors.As(e, &d) && d.Status == 422 {
		return acceptanceBlock(out, d.Message)
	}
	return out, e
}
func (c *acceptanceSourceContext) Source(ctx context.Context, p, id string) (domain.AcceptanceSource, error) {
	kit, current, e := c.p.Work.Inspect(ctx, p, id)
	if e != nil {
		return domain.AcceptanceSource{}, acceptanceMapped(e)
	}
	out := domain.AcceptanceSource{ProjectID: p, PackageID: id, Available: current.Available && current.Eligible, Blockers: append([]string{}, current.Blockers...)}
	for _, step := range kit.Steps {
		if step.Number == 10 && step.State == "blocked" {
			out.Blockers = append(out.Blockers, "Vendor acceptance has an unresolved blocker")
		}
	}
	order, e := c.p.Orders.Released(ctx, p, id)
	if e != nil {
		mappedErr := acceptanceMapped(e)
		var d *domain.AcceptanceError
		if errors.As(mappedErr, &d) && (d.Status == 404 || d.Status == 422) {
			return acceptanceBlock(out, "Release the actual purchase order before recording vendor acceptance")
		}
		return out, mappedErr
	}
	if order == nil || order.ReleasedAt == nil {
		return acceptanceBlock(out, "Release the actual purchase order before recording vendor acceptance")
	}
	out.Order = &domain.AcceptanceOrder{ID: order.ID, ProjectID: order.ProjectID, PackageID: order.PackageID, Version: order.Version, Number: order.Number, Date: order.Details.Date, SupplierID: order.Source.Supplier.ID, SupplierName: order.Source.Supplier.Name, Description: order.Source.Description, Quantity: order.Source.Quantity, UOM: order.Source.UOM, Currency: order.Source.Currency, Total: order.Source.Total, PaymentTerms: order.Source.PaymentTerms, DeliveryDays: order.Source.DeliveryDays, Warranty: order.Source.Warranty, ReleasedAt: *order.ReleasedAt, ReleasedBy: order.ReleasedBy}
	return out, nil
}
