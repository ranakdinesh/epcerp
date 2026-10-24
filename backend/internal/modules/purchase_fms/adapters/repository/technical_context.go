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

type TechnicalProviders struct {
	Clearances contracts.QuotationProjectClearance
	Projects   projects.ContextReader
	Purchase   contracts.WorkspaceAccessGate
	Access     access.Gate
	Users      auth.AssignmentDirectory
	Work       contracts.WorkflowSourceReader
	Released   contracts.SourcingReleasedReader
}
type technicalSourceContext struct{ p TechnicalProviders }

func NewTechnicalContext(p TechnicalProviders) *technicalSourceContext {
	return &technicalSourceContext{p}
}
func technicalMapped(e error) error {
	var a *access.Error
	if errors.As(e, &a) {
		return &domain.TechnicalError{Status: a.Status, Field: a.Field, Message: a.Message}
	}
	var w *contracts.WorkflowError
	if errors.As(e, &w) {
		return &domain.TechnicalError{Status: w.Status, Field: w.Field, Message: w.Message}
	}
	var s *contracts.SourcingError
	if errors.As(e, &s) {
		return &domain.TechnicalError{Status: s.Status, Field: s.Field, Message: s.Message}
	}
	var p *contracts.WorkspaceInvalid
	if errors.As(e, &p) {
		return domain.TechnicalInvalid(p.Field, p.Message)
	}
	if errors.Is(e, contracts.WorkspaceErrForbidden) {
		return domain.TechnicalForbidden
	}
	if errors.Is(e, contracts.WorkspaceErrMissing) {
		return domain.TechnicalMissing
	}
	if errors.Is(e, contracts.WorkspaceErrUnauthorized) {
		return domain.TechnicalUnauthorized
	}
	return e
}
func technicalCan(a domain.Principal, code string) bool {
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
func (c *technicalSourceContext) scoped(ctx context.Context, a domain.Principal, p, code string, write bool) (bool, error) {
	v, e := c.p.Purchase.Check(ctx, contracts.WorkspacePrincipal{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	if e == nil {
		return !write || v.CanWrite, nil
	}
	e = technicalMapped(e)
	var d *domain.TechnicalError
	if !errors.As(e, &d) || d.Status != 403 {
		return false, e
	}
	x, e := c.p.Access.Check(ctx, access.Actor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	e = technicalMapped(e)
	if errors.As(e, &d) && d.Status == 403 {
		return false, nil
	}
	return e == nil && (!write || x.CanWrite), e
}
func (c *technicalSourceContext) Access(ctx context.Context, a domain.Principal, p, id string) (out domain.TechnicalAccess, e error) {
	out.Work = map[int]bool{}
	parent, e := c.p.Projects.Get(ctx, p, true)
	if e != nil {
		return out, e
	}
	if parent == nil {
		return out, domain.TechnicalMissing
	}
	u, e := c.p.Users.Get(ctx, a.ID)
	if e != nil {
		return out, e
	}
	if u == nil || !u.Active {
		return out, domain.TechnicalUnauthorized
	}
	a.Superuser = a.Superuser && u.Superuser
	live := func(code string) bool { return technicalCan(a, code) && u.Allows(code) }
	if !live("projects.purchase_technical.read") {
		return out, nil
	}
	out.Scoped, e = c.scoped(ctx, a, p, "projects.purchase_technical.read", false)
	if e != nil {
		return out, e
	}
	out.Read = out.Scoped
	wa := contracts.WorkflowActor{ID: a.ID, Active: true, Superuser: a.Superuser, Permissions: a.Permissions}
	for _, n := range []int{6, 7, 8} {
		allowed, err := c.p.Work.CanWork(ctx, wa, p, id, n)
		if err != nil {
			var d *domain.TechnicalError
			err = technicalMapped(err)
			if !errors.As(err, &d) || d.Status != 422 {
				return out, err
			}
		}
		out.Work[n] = allowed
		taskRead, err := c.p.Work.CanReadTask(ctx, wa, p, id, n)
		if err != nil {
			return out, technicalMapped(err)
		}
		out.Read = out.Read || taskRead
	}
	out.Write = out.Read && live("projects.purchase_technical.write")
	if out.Scoped && live("projects.purchase_technical.approve") {
		out.Approve, e = c.scoped(ctx, a, p, "projects.purchase_technical.approve", true)
		if e != nil {
			var d *domain.TechnicalError
			if errors.As(e, &d) && d.Status == 422 {
				out.Approve = false
				e = nil
			}
		}
	}
	return out, e
}
func (c *technicalSourceContext) Source(ctx context.Context, p, id string) (domain.TechnicalSource, error) {
	kit, current, e := c.p.Work.Inspect(ctx, p, id)
	if e != nil {
		return domain.TechnicalSource{}, technicalMapped(e)
	}
	if c.p.Clearances != nil {
		ready, e := c.p.Clearances.Ready(ctx, p)
		if e != nil {
			return domain.TechnicalSource{}, technicalMapped(e)
		}
		if !ready {
			return domain.TechnicalSource{ProjectID: p, PackageID: id, Blockers: []string{"Finance must clear every current purchase group before technical documents"}}, nil
		}
	}
	loi, e := c.p.Released.Released(ctx, p, id)
	if e != nil {
		return domain.TechnicalSource{}, technicalMapped(e)
	}
	out := domain.TechnicalSource{ProjectID: p, PackageID: id, Available: current.Available && current.Eligible, Blockers: append([]string{}, current.Blockers...)}
	if loi == nil {
		out.Available = false
		out.Blockers = append(out.Blockers, "Release an independently approved LOI before creating the technical kit")
		return out, nil
	}
	out.CaseID = loi.CaseID
	out.Version = loi.Version
	out.LOIReference = loi.LOIReference
	out.SupplierID = loi.Offer.SupplierID
	out.SupplierName = loi.Offer.Supplier.Name
	out.Description = loi.Source.Description
	out.Quantity = loi.Source.Quantity
	out.Currency = loi.Source.Currency
	out.Total = loi.Total
	out.FinancialReference = loi.FinancialReference
	out.ReleasedDate = domain.TechnicalToday(loi.ReleasedAt)
	if !current.Available {
		out.Blockers = append(out.Blockers, "Current project controls prevent technical execution")
	}
	for _, step := range kit.Steps {
		if step.Number >= 6 && step.Number <= 8 && step.State == "blocked" {
			out.Blockers = append(out.Blockers, "Technical preparation has an unresolved blocker")
		}
	}
	return out, nil
}
